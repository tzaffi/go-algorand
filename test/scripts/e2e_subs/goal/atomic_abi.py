import json
from subprocess import call
from typing import Callable, Dict, List, Union, Tuple
from pathlib import Path
import types

from goal import Goal, text

import algosdk.atomic_transaction_composer as atc
import algosdk.abi as abi
import algosdk.future.transaction as txn


class AtomicABI:
    """
    AtomicABI allows for easier usage of ABI methods in Python.

    For an abi method such as `factorial(uint64)uint64`
    this allows usages such as:

    >>> result = abi.run_factorial(5)
    which will:
    * create an atomic transaction composer
    * add a single metod

    or for an atomic transaction group, and another method
    such as `add(uint64,uint64)uint64`
    you build up the group with:

    >>> abi.next_abi_call_factorial(5)
    >>> abi.next_abi_call_add(29, 13)

    and then execute the group with:

    >>> abi.execute_atomic_group()
    """

    CALL_TWICE_ERROR = "Cannot execute this Atomic ABI twice. Instantiate a new object to execute again."

    def __init__(
        self,
        goal: Goal,
        app_id: int,
        contract_abi_json: Union[Path, str],
        caller_acct: str,
        sp: txn.SuggestedParams = None,
    ):
        """
        Note: app_id will over-write whatever app_id was defined in `contract_abi_json`

        Also, we're assuming a single caller_account which is also the signer for all the transactions.
        """
        self.goal = goal
        self.app_id = app_id
        self.contract_abi_json = contract_abi_json
        self.caller_acct = caller_acct
        self.sp = sp

        assert (
            self.app_id
        ), f"must have already created the app but have app_id {self.app_id}"

        assert (
            self.caller_acct in self.goal.internal_wallet
        ), "aborting AtomicABI - will not be able to transact without signing authority"

        self.contract_abi_json_path: str = None
        try:
            cajson = open(contract_abi_json, "rt").read().strip()
            self.contract_abi_json_path = contract_abi_json
        except Exception:
            cajson = contract_abi_json

        cadict = json.loads(cajson)
        cadict["appId"] = self.app_id
        self.contract: abi.Contract = abi.Contract.from_json(json.dumps(cadict))

        self.sp = sp
        assert (
            self.caller_acct
        ), "aborting AtomicABI - cannot execute without a caller_acct"
        self.signer = self.get_atxn_signer()

        # list of lists of method_args for each of the methods to be called:
        self.method_args: List[list] = []

        self.sig2selector: Dict[str, str] = {}
        self._meth_dict: Dict[str, dict] = {}

        self.execution_results: atc.AtomicTransactionResponse = None
        self.execution_summaries: List[MethodCallSummary] = None

        self.atomic_transaction_composer = atc.AtomicTransactionComposer()

        for abi_meth in self.contract.methods:
            (
                handle,
                am_name,
                adder_meth,
                rn_name,
                run_now_meth,
            ) = self._attach_dynamic_method_calls(
                abi_meth.name, *self._method_factories(abi_meth)
            )
            signature = abi_meth.get_signature()
            selector = "0x" + abi_meth.get_selector().hex()
            self.sig2selector[signature] = selector
            self._meth_dict[handle] = {
                "signature": signature,
                "selector": selector,
                "abi_meth": abi_meth,
                "adder_meth_name": am_name,
                "adder_meth": adder_meth,
                "run_now_meth_name": rn_name,
                "run_now_meth": run_now_meth,
            }

    @classmethod
    def _clone(
        cls, obj, caller_acct: str = None, new_suggested_params: bool = True
    ) -> "AtomicABI":
        """
        new_suggested_params defaults to True because we don't want an intentionally
        cloned transaction to clash with a previous one (unless it's in the same round)
        """
        sp = obj.goal.algod.suggested_params() if new_suggested_params else obj.sp
        return cls(
            obj.goal,
            obj.app_id,
            obj.contract_abi_json,
            caller_acct if caller_acct else obj.caller_acct,
            sp=sp,
        )

    def clone(
        self, caller_acct: str = None, new_suggested_params: bool = True
    ) -> "AtomicABI":
        return self._clone(
            self, caller_acct=caller_acct, new_suggested_params=new_suggested_params
        )

    def execute_atomic_group(self, wait_rounds: int = 5) -> list:
        assert self.execution_results is None, self.CALL_TWICE_ERROR

        self.execution_results = self.atomic_transaction_composer.execute(
            self.goal.algod, wait_rounds
        )
        self.execution_summaries = self._build_summaries()
        return list(map(lambda y: y.result.return_value, self.execution_summaries))

    def execute_singleton_group(
        self,
        method_handle: str,
        method_args: list,
        wait_rounds: int = 5,
        sp: txn.SuggestedParams = None,
        on_complete: txn.OnComplete = txn.OnComplete.NoOpOC,
        note: bytes = None,
        lease: bytes = None,
        rekey_to: str = None,
    ) -> object:
        """
        Note: the run_XYZ() dynamically generated methods are recommended over execute_singleton_group()
        in most situation because they clone() the AtomicABI object first before execution.

        However, in cases when method signatures are not known a-priori and only a single method
        needs to be executed, execute_singleton_group() is needed.
        """
        assert self.execution_results is None, self.CALL_TWICE_ERROR
        abi_meth = self._meth_dict[method_handle]["abi_meth"]
        self.add_method_call(
            abi_meth,
            method_args,
            sp=sp,
            on_complete=on_complete,
            note=note,
            lease=lease,
            rekey_to=rekey_to,
        )
        return self.execute_atomic_group(wait_rounds=wait_rounds)[0]

    def dump_selectors(self) -> str:
        return json.dumps(self.sig2selector, indent=4, sort_keys=True)

    def _build_summaries(self) -> List["MethodCallSummary"]:
        assert (
            self.execution_results
        ), "Cannot summarize before calling 'execute_atomic_group()'"
        summaries = []
        i = 0
        for meth in self.atomic_transaction_composer.method_dict.values():
            summaries.append(
                MethodCallSummary(
                    meth,
                    self.method_args[i],
                    self.execution_results.abi_results[i],
                )
            )
            i += 1
        return summaries

    @staticmethod
    def _method_factories(abi_meth: abi.method.Method) -> Tuple[Callable, Callable]:
        def func_add_method_call(
            self,
            *args,
            sp: txn.SuggestedParams = None,
            on_complete: txn.OnComplete = txn.OnComplete.NoOpOC,
            note: bytes = None,
            lease: bytes = None,
            rekey_to: str = None,
        ):
            return self.add_method_call(
                abi_meth,
                method_args=args,
                sp=sp,
                on_complete=on_complete,
                note=note,
                lease=lease,
                rekey_to=rekey_to,
            )

        def func_run_now(
            self,
            *args,
            wait_rounds: int = 5,
            sp: txn.SuggestedParams = None,
            on_complete: txn.OnComplete = txn.OnComplete.NoOpOC,
            note: bytes = None,
            lease: bytes = None,
            rekey_to: str = None,
        ):
            self.add_method_call(
                abi_meth,
                method_args=args,
                sp=sp,
                on_complete=on_complete,
                note=note,
                lease=lease,
                rekey_to=rekey_to,
            )
            return self.execute_atomic_group(wait_rounds=wait_rounds)[0]

        return func_add_method_call, func_run_now

    def get_suggested_params(self) -> txn.SuggestedParams:
        if not self.sp:
            self.sp = self.goal.algod.suggested_params()

        return self.sp

    def get_atxn_signer(self, caller_acct: str = None) -> atc.AccountTransactionSigner:
        if not caller_acct:
            caller_acct = self.caller_acct
        sk = self.goal.internal_wallet.get(caller_acct)
        if not sk:
            raise Exception("Cannot create AccountTransactionSigner")
        # TODO: handle querying kmd in the case that sk isn't in the internal wallet

        return atc.AccountTransactionSigner(sk)

    def get_txn_with_signer(
        self, txn: txn.Transaction, signer: atc.TransactionSigner = None
    ) -> atc.TransactionWithSigner:
        if not signer:
            signer = self.signer

        return atc.TransactionWithSigner(txn, signer)

    def add_method_call(
        self,
        method: abi.method.Method,
        method_args: list = [],
        sp: txn.SuggestedParams = None,
        on_complete: txn.OnComplete = txn.OnComplete.NoOpOC,
        note: bytes = None,
        lease: bytes = None,
        rekey_to: str = None,
    ) -> "AtomicABI":
        assert self.execution_results is None, self.CALL_TWICE_ERROR

        if not sp:
            sp = self.get_suggested_params()

        self.atomic_transaction_composer.add_method_call(
            self.app_id,
            method,
            self.caller_acct,
            sp,
            self.signer,
            method_args=method_args,
            on_complete=on_complete,
            note=note,
            lease=lease,
            rekey_to=rekey_to,
        )

        self.method_args.append(method_args)

        return self

    def _attach_dynamic_method_calls(
        self, name: str, adder_func: Callable, run_now_func: Callable
    ) -> tuple:
        adder_meth = types.MethodType(adder_func, self)
        adder_meth_name = self.abi_composer_name(name)
        setattr(self, adder_meth_name, adder_meth)

        run_now_meth = types.MethodType(run_now_func, self)
        rn_meth_name = self.run_now_method_name(name)
        setattr(self, rn_meth_name, run_now_meth)

        return name, adder_meth_name, adder_meth, rn_meth_name, run_now_meth

    @classmethod
    def abi_composer_name(cls, method_name: str) -> str:
        return f"next_abi_call_{method_name}"

    @classmethod
    def run_now_method_name(cls, method_name: str) -> str:
        return f"run_{method_name}"


class MethodCallSummary:
    def __init__(self, method: abi.Method, args: list, result: atc.ABIResult):
        self.method = method
        self.args = args
        self.result = result

    def __str__(self) -> str:
        return f"""SELECTOR<<<0x{self.method.get_selector().hex()}>>>
{self.method.get_signature()}: {self.args} 
    ->
{self.result.return_value}"""