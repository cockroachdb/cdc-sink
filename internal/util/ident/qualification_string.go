// Code generated by "stringer -type=Qualification"; DO NOT EDIT.

package ident

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[TableOnly-1]
	_ = x[PartialSchema-2]
	_ = x[FullyQualified-3]
}

const _Qualification_name = "TableOnlyPartialSchemaFullyQualified"

var _Qualification_index = [...]uint8{0, 9, 22, 36}

func (i Qualification) String() string {
	i -= 1
	if i < 0 || i >= Qualification(len(_Qualification_index)-1) {
		return "Qualification(" + strconv.FormatInt(int64(i+1), 10) + ")"
	}
	return _Qualification_name[_Qualification_index[i]:_Qualification_index[i+1]]
}
