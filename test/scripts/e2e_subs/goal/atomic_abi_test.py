import json
from os import confstr_names
from typing import List
from unittest.mock import Mock, patch

import algosdk.atomic_transaction_composer as atc
import algosdk.abi as abi
import algosdk.future.transaction as txn

from .atomic_abi import AtomicABI


contract = {
    "name": "demo-abi",
    "appId": None,
    "methods": [
        {
            "name": "add",
            "desc": "Add 2 integers",
            "args": [{"type": "uint64"}, {"type": "uint64"}],
            "returns": {"type": "uint64"},
        },
        {
            "name": "sub",
            "desc": "Subtract 2 integers",
            "args": [{"type": "uint64"}, {"type": "uint64"}],
            "returns": {"type": "uint64"},
        },
        {
            "name": "mul",
            "desc": "Multiply 2 integers",
            "args": [{"type": "uint64"}, {"type": "uint64"}],
            "returns": {"type": "uint64"},
        },
        {
            "name": "div",
            "desc": "Divide 2 integers, throw away the remainder",
            "args": [{"type": "uint64"}, {"type": "uint64"}],
            "returns": {"type": "uint64"},
        },
        {
            "name": "qrem",
            "desc": "Divide 2 integers, return both the quotient and remainder",
            "args": [{"type": "uint64"}, {"type": "uint64"}],
            "returns": {"type": "(uint64,uint64)"},
        },
        {
            "name": "reverse",
            "desc": "Reverses a string",
            "args": [{"type": "string"}],
            "returns": {"type": "string"},
        },
        {
            "name": "txntest",
            "desc": "just check it",
            "args": [{"type": "uint64"}, {"type": "pay"}, {"type": "uint64"}],
            "returns": {"type": "uint64"},
        },
        {
            "name": "concat_strings",
            "desc": "concat some strings",
            "args": [{"type": "string[]"}],
            "returns": {"type": "string"},
        },
        {
            "name": "manyargs",
            "desc": "Try to send 20 arguments",
            "args": [
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
                {"type": "uint64"},
            ],
            "returns": {"type": "uint64"},
        },
        {
            "name": "_optIn",
            "desc": "just opt in",
            "args": [{"type": "uint64"}],
            "returns": {"type": "uint64"},
        },
        {
            "name": "_closeOut",
            "desc": "just close out",
            "args": [{"type": "uint64"}],
            "returns": {"type": "uint64"},
        },
    ],
}


def test_fixture():
    num_methods = len(contract["methods"])
    assert num_methods == 11
    assert json.loads(json.dumps(contract))["appId"] is None


def test_init(init_only=False):
    goal = Mock()
    caller_account = "mega whale"
    sk = Mock()
    goal.internal_wallet = {caller_account: sk}

    app_id = 42
    contract_abi_json = json.dumps(contract)
    sp = Mock()
    abi = AtomicABI(goal, app_id, contract_abi_json, caller_account, sp=sp)
    if init_only:
        return abi

    assert abi.app_id == app_id
    assert abi.caller_acct == caller_account
    assert abi.sp == sp

    assert abi.contract_abi_json == contract_abi_json
    assert abi.contract.name == "demo-abi"
    assert abi.contract.app_id == app_id

    assert abi.signer.private_key == sk
    num_methods = len(contract["methods"])
    assert num_methods == len(abi.contract.methods)


def test_dynamic_methods():
    abi = test_init(init_only=True)
    for meth in contract["methods"]:
        name = meth["name"]
        adder_meth_name = abi.abi_composer_name(name)
        assert getattr(abi, adder_meth_name, None)

        run_now_method_name = abi.run_now_method_name(name)
        assert getattr(abi, run_now_method_name, None)


ZERO_ADDR = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAY5HFKQ"


def zero_val(val: abi.Returns) -> object:
    return zero(val.type)


def zero(t: abi.ABIType) -> object:
    if isinstance(t, abi.UintType):
        return 0

    if isinstance(t, abi.StringType):
        return ""

    if isinstance(t, abi.TupleType):
        return tuple(map(zero, t.child_types))

    if isinstance(t, abi.ArrayDynamicType):
        return list(map(zero, t.child_types))

    # if t == "pay":
    #     pymnt = txn.PaymentTxn(ZERO_ADDR, Mock(), ZERO_ADDR, 0)
    #     signer = atc.AccountTransactionSigner("")
    #     return atc.TransactionWithSigner(pymnt, signer)

    # if t.endswith("[]"):
    #     return list()

    raise Exception(f"unhandled type <{t}>")


def make_atc_response(*methods: List[abi.method.Method]):
    confirmed_round = 1337
    tx_ids = list(map(lambda m: f"txn for {m.name}", methods))
    method_results = []
    for i, meth in enumerate(methods):
        method_results.append(
            atc.ABIResult(tx_ids[i], None, zero_val(meth.returns), None)
        )

    atc_response = atc.AtomicTransactionResponse(
        confirmed_round=confirmed_round, tx_ids=tx_ids, results=method_results
    )

    return atc_response


def test_run_methods():
    abi = test_init(init_only=True)
    with patch.object(atc.AtomicTransactionComposer, "execute") as atc_execute:
        atc_execute.return_value = make_atc_response(abi._meth_dict["add"]["abi_meth"])

        z = abi.run_add(2, 3)
    assert z == 0

    assert len(abi.execution_results.abi_results) == 1
    assert abi.execution_results.abi_results[0].return_value == 0

    assert len(abi.execution_results.tx_ids) == 1
    assert abi.execution_results.tx_ids[0] == "txn for add"

    assert len(abi.execution_summaries) == 1
    assert abi.execution_summaries[0].args == (2, 3)
    assert abi.execution_summaries[0].result.return_value == 0


def test_execute_atomic_group():
    abi = test_init(init_only=True)
    mnames = [
        "add",
        "sub",
        "mul",
        "div",
        "qrem",
        "reverse",
        "txntest",
        "concat_strings",
        "manyargs",
        "_optIn",
        "_closeOut",
    ]
    responses = [abi._meth_dict[m]["abi_meth"] for m in mnames]

    with patch.object(atc.AtomicTransactionComposer, "execute") as atc_execute:
        atc_execute.return_value = make_atc_response(*responses)
        abi.next_abi_call_add(1, 2)
        abi.next_abi_call_sub(2, 1)
        abi.next_abi_call_mul(4, 5)
        abi.next_abi_call_div(12, 2)
        abi.next_abi_call_qrem(43, 5)
        abi.next_abi_call_reverse("allo")

        pymnt = txn.PaymentTxn(ZERO_ADDR, Mock(), ZERO_ADDR, 0)
        signed_pymnt = abi.get_txn_with_signer(pymnt)
        abi.next_abi_call_txntest(42, signed_pymnt, 24)

        abi.next_abi_call_concat_strings(["by", "bye"])
        abi.next_abi_call_manyargs(*range(20))
        abi.next_abi_call__optIn(0)
        abi.next_abi_call__closeOut(0)

        y = abi.execute_atomic_group()

    num_calls = 11

    assert len(y) == num_calls
    assert len(abi.execution_results.abi_results) == num_calls
    assert len(abi.execution_results.tx_ids) == num_calls
    assert len(abi.execution_summaries) == num_calls

    for i, summary in enumerate(abi.execution_summaries):
        abi_result = abi.execution_results.abi_results[i]
        faux_txn = abi.execution_results.tx_ids[i]

        assert faux_txn == abi_result.tx_id
        assert summary.method.name in faux_txn
        assert summary.result == abi_result
        assert y[i] == abi_result.return_value

    # how'd we do with add() ?
    add_idx = 0
    add_exp_result = 0
    assert abi.execution_summaries[add_idx].args == (1, 2)
    assert abi.execution_summaries[add_idx].result.return_value == add_exp_result
    assert abi.execution_results.tx_ids[add_idx] == "txn for add"
    assert abi.execution_results.abi_results[add_idx].return_value == add_exp_result
    assert y[add_idx] == add_exp_result

    # how'd we do with qrem() ?
    qrem_idx = 4
    qrem_exp_result = (0, 0)
    assert abi.execution_summaries[qrem_idx].args == (43, 5)
    assert abi.execution_summaries[qrem_idx].result.return_value == qrem_exp_result
    assert abi.execution_results.tx_ids[qrem_idx] == "txn for qrem"
    assert abi.execution_results.abi_results[qrem_idx].return_value == qrem_exp_result
    assert y[qrem_idx] == qrem_exp_result

    # how'd we do with reverse() ?
    reverse_idx = 5
    reverse_exp_result = ""
    assert abi.execution_summaries[reverse_idx].args == ("allo",)
    assert (
        abi.execution_summaries[reverse_idx].result.return_value == reverse_exp_result
    )
    assert abi.execution_results.tx_ids[reverse_idx] == "txn for reverse"
    assert (
        abi.execution_results.abi_results[reverse_idx].return_value
        == reverse_exp_result
    )
    assert y[reverse_idx] == reverse_exp_result

    # how'd we do with txntest() ?
    txntest_idx = 6
    txntest_exp_result = 0
    assert abi.execution_summaries[txntest_idx].args == (42, signed_pymnt, 24)

    assert (
        abi.execution_summaries[txntest_idx].result.return_value == txntest_exp_result
    )
    assert abi.execution_results.tx_ids[txntest_idx] == "txn for txntest"
    assert (
        abi.execution_results.abi_results[txntest_idx].return_value
        == txntest_exp_result
    )
    assert y[txntest_idx] == txntest_exp_result

    # how'd we do with concat_strings() ?
    concat_strings_idx = 7
    concat_strings_exp_result = ""
    assert abi.execution_summaries[concat_strings_idx].args == (["by", "bye"],)

    assert (
        abi.execution_summaries[concat_strings_idx].result.return_value
        == concat_strings_exp_result
    )
    assert abi.execution_results.tx_ids[concat_strings_idx] == "txn for concat_strings"
    assert (
        abi.execution_results.abi_results[concat_strings_idx].return_value
        == concat_strings_exp_result
    )
    assert y[concat_strings_idx] == concat_strings_exp_result

    # how'd we do with manyargs() ?
    manyargs_idx = 8
    manyargs_exp_result = 0
    assert abi.execution_summaries[manyargs_idx].args == tuple(range(20))

    assert (
        abi.execution_summaries[manyargs_idx].result.return_value == manyargs_exp_result
    )
    assert abi.execution_results.tx_ids[manyargs_idx] == "txn for manyargs"
    assert (
        abi.execution_results.abi_results[manyargs_idx].return_value
        == manyargs_exp_result
    )
    assert y[manyargs_idx] == manyargs_exp_result
