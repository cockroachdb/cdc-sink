// Code generated by "stringer -type=operationType"; DO NOT EDIT.

package cdc

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[unknownOp-0]
	_ = x[deleteOp-1]
	_ = x[insertOp-2]
	_ = x[updateOp-3]
	_ = x[upsertOp-4]
}

const _operationType_name = "unknownOpdeleteOpinsertOpupdateOpupsertOp"

var _operationType_index = [...]uint8{0, 9, 17, 25, 33, 41}

func (i operationType) String() string {
	if i < 0 || i >= operationType(len(_operationType_index)-1) {
		return "operationType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _operationType_name[_operationType_index[i]:_operationType_index[i+1]]
}
