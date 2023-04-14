package main

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	// xpkg "{{.XModulePath}}/{{.XPackagePath}}"  					/* TEMPLATE ONLY  */
	// ypkg "{{.YModulePath}}/{{.YPackagePath}}"  					/* TEMPLATE ONLY  */
	// ypkg "github.com/algorand/go-algorand-sdk/v2/types"      // 	/* GENERATOR ONLY */
	xpkg "github.com/algorand/go-algorand/ledger/ledgercore" // 	/* GENERATOR ONLY */
	ypkg "github.com/algorand/go-algorand/ledger/ledgercore" // 	/* GENERATOR ONLY */  <--- this one is just for getting the build to pass
)

// diffExclusions is a set of types to exclude. The string keys are assumed to be generated by
// the `Type.String()` method, and these values are ignored when building up the diff.
// These exclusions represent types that are known to serialize in the same way, but are
// would not do so without special treatment.
// For example, `MicrAlgos` are a struct in go-algorand but a uint64 in go-algorand-sdk.
var diffExclusions = map[string]bool{
	`github.com/algorand/go-algorand/data/basics :: "basics.MicroAlgos" (struct)`: true,
}

func Main() { // replaced by main() in `make template4xrt`
	// x := reflect.TypeOf(xpkg.{{.XTypeInstance}}{}) 		 //  	/* TEMPLATE ONLY  */
	// y := reflect.TypeOf(ypkg.{{.YTypeInstance}}{}) 		 //  	/* TEMPLATE ONLY  */
	x := reflect.TypeOf(xpkg.StateDelta{}) //					    /* GENERATOR ONLY */
	// y := reflect.TypeOf(ypkg.LedgerStateDelta{}) //				/* GENERATOR ONLY */
	y := reflect.TypeOf(ypkg.StateDelta{}) //						/* GENERATOR ONLY */ <--- this one is just for getting the build to pass

	// ---- BUILD ---- //

	xRoot := Type{Type: x, Kind: x.Kind()}
	fmt.Printf("Build the Type Tree for %s\n\n", &xRoot)
	xRoot.Build()
	xTgt := Target{Edge{Name: fmt.Sprintf("%q", x)}, xRoot}

	yRoot := Type{Type: y, Kind: y.Kind()}
	fmt.Printf("Build the Type Tree for %s\n\n", &yRoot)
	yRoot.Build()
	yTgt := Target{Edge{Name: fmt.Sprintf("%q", y)}, yRoot}

	// ---- DEBUG ---- //

	/*
		xRoot.Print()
		fmt.Printf("\n\nSerialization Tree of %q\n\n", x)
		xTgt.PrintSerializable()

		yRoot.Print()
		fmt.Printf("\n\nSerialization Tree of %q\n\n", x)
		yTgt.PrintSerializable()
	*/

	// ---- STATS ---- //

	LeafStatsReport(xTgt)
	LeafStatsReport(yTgt)

	// ---- DIFF ---- //

	fmt.Printf("\n\nCompare the Type Trees %q v %q\n", x, y)
	diff, err := SerializationDiff(xTgt, yTgt, diffExclusions)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}
	Report(xTgt, yTgt, diff)

	if !diff.Empty() {
		// signal the this "test" has failed
		os.Exit(1)
	}
}

// --------------- TYPE TREE DATA STRUCTURES --------------- //

// Type wraps reflect.Type and reflect.Kind to make it easier to
// build a tree of types.
type Type struct {
	Depth    int
	Type     reflect.Type
	Kind     reflect.Kind
	Edges    []Edge
	children *Children
}

func (t *Type) String() string {
	return fmt.Sprintf("%s :: %q (%s)", t.Type.PkgPath(), t.Type, t.Kind)
}

// Children represent a Type's child Types.
type Children map[string]Type

// Edge represents the name a type is referred together with
// the Tag metadata (for the case of a struct field)
type Edge struct {
	Name, Tag string
}

// String represents the Edge as in an invertible manner.
func (e *Edge) String() string {
	return fmt.Sprintf("[%s](%s)", e.Name, e.Tag)
}

// EdgeFromLabel de-"serialize" and Edge that has be rendered via Edge.String()
func EdgeFromLabel(s string) *Edge {
	re := regexp.MustCompile(`^\[(.+)\]\((.+)\)$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) == 3 {
		return &Edge{Name: matches[1], Tag: matches[2]}
	}
	return nil
}

// Target represents a Type that is a child of another Type by
// providing the Edge that "points" to it.
// In the case of a "root" Type, it's up to the caller to provide
// the Edge.
type Target struct {
	Edge
	Type Type
}

// String is a convenience method for printing a Target.
func (tgt *Target) String() string {
	return fmt.Sprintf("%s|-->%s", &tgt.Edge, &tgt.Type)
}

// SerializationInfo provides the essential data
// for go to serialize the field or other type.
func (e *Edge) SerializationInfo() string {
	// Probably more subtelty is called for.
	re := regexp.MustCompile(`^codec:"([^,"]+)`)

	if e.Tag == "" {
		return e.Name
	}

	matches := re.FindStringSubmatch(e.Tag)
	if len(matches) > 1 {
		return matches[1]
	}
	return e.Tag
}

// Diff represents a difference (if any) between two Types.
// CommonPath is the path in the tree from the root that arrives
// at the difference.
// Xdiff and Ydiff are the differences at the end of the common path
// between children. Set theoretically:
// * Xdiff = X - Y
// * Ydiff = Y - X.
type Diff struct {
	CommonPath   []Target
	Xdiff, Ydiff []Target
}

// Empty reports whethere there was any difference at all.
func (d *Diff) Empty() bool {
	return d == nil || (len(d.Xdiff) == 0 && len(d.Ydiff) == 0)
}

// --------------- BUILD THE TYPE TREE --------------- //

// Targets returns a slice of Targets, one for each child of the Type.
// In the case of structs, the order is the same as the order of the fields in the struct.
// In the case of maps, the key Target precedes the value Target.
func (t *Type) Targets() []Target {
	targets := make([]Target, 0, len(t.Edges))
	for _, edge := range t.Edges {
		targets = append(targets, Target{edge, (*t.children)[edge.String()]})
	}
	return targets
}

// IsLeaf returns true if the Type has no children.
func (t *Type) IsLeaf() bool {
	return t.children == nil
}

// Build the Type tree by finding all child types that belong to a kind and recursively
// building their Type trees.
func (t *Type) Build() {
	switch t.Kind {
	case reflect.Struct:
		t.buildStructChildren()
	case reflect.Slice, reflect.Array:
		t.buildListChild()
	case reflect.Map:
		t.buildMapChildren()
	case reflect.Ptr:
		t.buildPtrChild()
	}
}

func (t *Type) appendChild(typeName, typeTag string, child Type) {
	edge := Edge{typeName, typeTag}
	t.Edges = append(t.Edges, edge)
	if t.children == nil {
		children := make(Children)
		t.children = &children
	}
	(*t.children)[edge.String()] = child
}

func (t *Type) buildStructChildren() {
	for i := 0; i < t.Type.NumField(); i++ {
		typeField := t.Type.Field(i)
		typeName := typeField.Name

		// probably we need to skip typeField.Tag == `codec:"-"` as well
		if typeName == "" || (!unicode.IsUpper(rune(typeName[0])) && typeName != "_struct") {
			continue
		}

		typeTag := string(typeField.Tag)
		child := Type{t.Depth + 1, typeField.Type, typeField.Type.Kind(), nil, nil}
		child.Build()
		t.appendChild(typeName, typeTag, child)
	}
}

func (t *Type) buildListChild() {
	tt := t.Type.Elem()
	child := Type{t.Depth + 1, tt, tt.Kind(), nil, nil}
	child.Build()
	t.appendChild("<list elt>", "", child)
}

func (t *Type) buildMapChildren() {
	keyType, valueType := t.Type.Key(), t.Type.Elem()

	keyChild := Type{t.Depth + 1, keyType, keyType.Kind(), nil, nil}
	keyChild.Build()
	t.appendChild("<map key>", "", keyChild)

	valChild := Type{t.Depth + 1, valueType, valueType.Kind(), nil, nil}
	valChild.Build()
	t.appendChild("<map elt>", "", valChild)
}

func (t *Type) buildPtrChild() {
	tt := t.Type.Elem()
	child := Type{t.Depth + 1, tt, tt.Kind(), nil, nil}
	child.Build()
	t.appendChild("<ptr elt>", "", child)
}

// Visit traverses the Target tree and applies any actions provided at each node.
func (tgt *Target) Visit(actions ...func(Target)) {
	if len(actions) > 0 {
		for _, action := range actions {
			action(*tgt)
		}
		for _, target := range tgt.Type.Targets() {
			target.Visit(actions...)
		}
	}
}

// SerializationDiff recursively computes a diff between two Target's x and y considering only data
// that impacts serialization, but ignoring any assumption on key ordering.
func SerializationDiff(x, y Target, exclusions map[string]bool) (*Diff, error) {
	xtype, ytype := x.Type, y.Type
	if xtype.Depth != ytype.Depth {
		return nil, fmt.Errorf("cannot compare types at different depth")
	}
	// if we got here it must be the case that either depth == 0 or
	// the edges of x and y serialize the same way.

	// So look at the children.
	// If any children differ report back the diff.
	xTgts, yTgts := xtype.Targets(), ytype.Targets()
	xSerials, ySerials := make(map[string]Target), make(map[string]Target)
	for _, tgt := range xTgts {
		xSerials[tgt.Edge.SerializationInfo()] = tgt
	}
	for _, tgt := range yTgts {
		ySerials[tgt.Edge.SerializationInfo()] = tgt
	}
	xDiff, yDiff := []Target{}, []Target{}
	for k, v := range xSerials {
		if _, ok := ySerials[k]; !ok {
			xDiff = append(xDiff, v)
		}
	}
	for k, v := range ySerials {
		if _, ok := xSerials[k]; !ok {
			yDiff = append(yDiff, v)
		}
	}
	if len(xDiff) != 0 || len(yDiff) != 0 {
		return &Diff{
			Xdiff: xDiff,
			Ydiff: yDiff,
		}, nil
	}

	// Otherwise, call the children recursively. If any of them report
	// a diff, modify the diff's CommonPath to include the current edge and return it.
	for k, xChild := range xSerials {
		if _, ok := exclusions[xChild.Type.String()]; ok {
			continue
		}
		yChild := ySerials[k]
		diff, err := SerializationDiff(xChild, yChild, exclusions)

		// TODO: Remve this debug code:
		x := fmt.Sprintf("%q", xChild.Type.Type)
		y := xChild.Type.String()
		_ = x
		_ = y

		if err != nil {
			return nil, err
		}
		if diff != nil {
			diff.CommonPath = append([]Target{xChild}, diff.CommonPath...)
			return diff, nil
		}
	}
	// No diffs detected up the tree:
	return nil, nil
}

// --------------- DIFF REPORT ----------------- //

// Report prints out a human readable listing of the differences.
func Report(x, y Target, d *Diff) {
	xType := x.Type
	yType := y.Type
	fmt.Printf(`

========================================================
			REPORT
comparing 
	<<<<<%s>>>>>
VS
	<<<<<%s>>>>>
========================================================
`, &xType, &yType)
	if d == nil {
		fmt.Println("No differences found.")
	} else {
		if len(d.Xdiff) == 0 && len(d.Ydiff) == 0 {
			if len(d.CommonPath) != 0 {
				panic("A common paths was found with no diffs. This should NEVER happen.")
			}
			fmt.Println("No differences found.")
		} else {
			fmt.Print(`
--------------------------------------------------------
		DIFFERENCES FOUND (partial)
--------------------------------------------------------
`)
			fmt.Printf("Common path of length %d:\n", len(d.CommonPath))
			for depth, tgt := range d.CommonPath {
				fmt.Printf("%s%s. SOURCE: %s\n", strings.Repeat(" ", depth), &tgt.Edge, &tgt.Type)
			}
			fmt.Printf(`
X-DIFF
------
EXISTS IN:	 	%q
MISSING FROM:		%q
%d TYPES TOTAL:
`, &xType, &yType, len(d.Xdiff))
			for i, tgt := range d.Xdiff {
				fmt.Printf(`
(%d)
[FIELD](+codec): 	%s
SOURCE: 		%s`, i+1, &tgt.Edge, &tgt.Type)
			}

			fmt.Printf(`



Y-DIFF
------
EXISTS IN:	 	%q
MISSING FROM:		%q
%d TYPES TOTAL:
`, &yType, &xType, len(d.Ydiff))
			for i, tgt := range d.Ydiff {
				fmt.Printf(`(%d)
[FIELD](+codec): 	%s
SOURCE: 		%s
`, i+1, &tgt.Edge, &tgt.Type)
			}
		}
	}
	fmt.Println(`
========================================================
===============        REPORT END        ===============
========================================================`)
}

// ------- STATISTICS AND DEBUGGING ------------- //

func (t *Type) Print() {
	action := func(tgt Target) {
		tabs := strings.Repeat("\t", tgt.Type.Depth)
		fmt.Printf("%s[depth=%d]. Value is type %q (%s)\n", tabs, tgt.Type.Depth, tgt.Type.Type, tgt.Type.Kind)

		if tgt.Type.IsLeaf() {
			fmt.Printf("%s-------B I N G O: A LEAF---------->%q (%s)\n", tabs, tgt.Type.Type, tgt.Type.Kind)
			return
		}
		fmt.Printf("%s=====EDGE: %s=====>\n", tabs, tgt.Edge)
	}
	(&Target{Edge{}, *t}).Visit(action)
}

// PrintSerializable prints the information that determines go-codec serialization.
// cf: https://github.com/algorand/go-codec/blob/master/codec/encode.go#L1416-L1436
func (tgt Target) PrintSerializable() {
	action := func(tgt Target) {
		ttype := tgt.Type
		tkind := ttype.Kind
		depth := ttype.Depth
		edge := tgt.Edge
		if depth == 0 {
			fmt.Printf("Serialization info for type %q (%s):\n", ttype.Type, tkind)
			return
		}
		fmt.Printf("%s%s", strings.Repeat(" ", depth-1), edge.SerializationInfo())
		suffix := ""
		if ttype.IsLeaf() {
			suffix = fmt.Sprintf(":%s", tkind)
		}
		fmt.Printf("%s\n", suffix)
	}
	tgt.Visit(action)
}

// LeafStatsReport prints out a report for the leafs type count.
func LeafStatsReport(xTgt Target) {
	fmt.Printf("\n\nLeaf-type stats for type %s:\n\n", &xTgt.Type)
	leaves := []Type{}
	leafCollector := func(tgt Target) {
		if tgt.Type.IsLeaf() {
			leaves = append(leaves, tgt.Type)
		}
	}

	xTgt.Visit(leafCollector)
	fmt.Printf("Found %d leaves\n\n", len(leaves))

	stats := make(map[string]int)
	for _, leaf := range leaves {
		key := fmt.Sprintf("%s/%s", leaf.Type, leaf.Kind)
		if _, ok := stats[key]; !ok {
			stats[key] = 0
		}
		stats[key]++
	}
	printSortedStats(stats)
}

type keyValue struct {
	Key   string
	Value int
}

func printSortedStats(stats map[string]int) {
	// Create a slice of key-value pairs
	var kvSlice []keyValue
	for k, v := range stats {
		kvSlice = append(kvSlice, keyValue{k, v})
	}

	// Sort the slice by the count in descending order
	sort.Slice(kvSlice, func(i, j int) bool {
		return kvSlice[i].Value > kvSlice[j].Value
	})

	// Print the sorted slice
	for _, kv := range kvSlice {
		fmt.Printf("%s: %d\n", kv.Key, kv.Value)
	}
}
