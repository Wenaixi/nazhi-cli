package types

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON 为 Task 的 hours 提供 number/string 双类型兼容，同时保持公开字段为 float64。
func (t *Task) UnmarshalJSON(data []byte) error {
	type taskAlias Task
	aux := struct {
		Hours json.RawMessage `json:"hours"`
		*taskAlias
	}{taskAlias: (*taskAlias)(t)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Hours != nil {
		var value FlexFloat
		if err := json.Unmarshal(aux.Hours, &value); err != nil {
			return fmt.Errorf("Task.hours: %w", err)
		}
		t.Hours = value.Float64()
	}
	return nil
}

// UnmarshalJSON 为 TaskCircleTypeInfo 的 hours 提供 number/string 双类型兼容。
func (t *TaskCircleTypeInfo) UnmarshalJSON(data []byte) error {
	type taskCircleTypeInfoAlias TaskCircleTypeInfo
	aux := struct {
		Hours json.RawMessage `json:"hours"`
		*taskCircleTypeInfoAlias
	}{taskCircleTypeInfoAlias: (*taskCircleTypeInfoAlias)(t)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Hours != nil {
		var value FlexFloat
		if err := json.Unmarshal(aux.Hours, &value); err != nil {
			return fmt.Errorf("TaskCircleTypeInfo.hours: %w", err)
		}
		t.Hours = value.Float64()
	}
	return nil
}

// UnmarshalJSON 为 CircleRecord 的 hours 提供 number/string 双类型兼容。
func (c *CircleRecord) UnmarshalJSON(data []byte) error {
	type circleRecordAlias CircleRecord
	aux := struct {
		Hours json.RawMessage `json:"hours"`
		*circleRecordAlias
	}{circleRecordAlias: (*circleRecordAlias)(c)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Hours != nil {
		var value FlexFloat
		if err := json.Unmarshal(aux.Hours, &value); err != nil {
			return fmt.Errorf("CircleRecord.hours: %w", err)
		}
		c.Hours = value.Float64()
	}
	return nil
}
