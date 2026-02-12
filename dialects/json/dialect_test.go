package r3json_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    *r3.FieldSpec
		expected string
	}{
		{
			name:     "valid field spec",
			input:    r3.NewFieldSpec("user_id"),
			expected: "user_id",
		},
		{
			name:     "nil field spec",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3json.FieldToJSON(tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestJSONToField(t *testing.T) {
	tests := []struct {
		name          string
		input         r3json.JSONField
		expectedField string
	}{
		{
			name:          "valid JSONField",
			input:         r3json.JSONField("id"),
			expectedField: "id",
		},
		{
			name:          "status field",
			input:         r3json.JSONField("status"),
			expectedField: "status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3json.JSONToField(tt.input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expectedField, result.String())
		})
	}
}

func TestJSONToFilter(t *testing.T) {
	tests := []struct {
		name        string
		input       *r3json.JSONFilter
		expectError bool
	}{
		{
			name: "valid JSONFilter",
			input: &r3json.JSONFilter{
				Field: r3json.JSONField("id"),
				Op:    r3json.OperatorEq,
				Value: 123,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3json.JSONToFilter(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestOperatorToJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       r3.FilterOperatorSpec
		expected    r3json.JSONFilterOperator
		expectError bool
	}{
		{name: "OperatorEq", input: r3.OperatorEq, expected: r3json.OperatorEq},
		{name: "OperatorNe", input: r3.OperatorNe, expected: r3json.OperatorNe},
		{name: "OperatorGt", input: r3.OperatorGt, expected: r3json.OperatorGt},
		{name: "OperatorLike", input: r3.OperatorLike, expected: r3json.OperatorLike},
		{name: "OperatorUnspecified", input: r3.OperatorUnspecified, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3json.OperatorToJSON(tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJSONToOperator(t *testing.T) {
	tests := []struct {
		name        string
		input       r3json.JSONFilterOperator
		expected    r3.FilterOperatorSpec
		expectError bool
	}{
		{name: "Eq", input: r3json.OperatorEq, expected: r3.OperatorEq},
		{name: "Gt", input: r3json.OperatorGt, expected: r3.OperatorGt},
		{name: "Unspecified", input: r3json.OperatorUnspecified, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3json.JSONToOperator(tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFilterToJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       *r3.FilterSpec
		expectError bool
		validate    func(t *testing.T, result *r3json.JSONFilter)
	}{
		{
			name: "valid simple filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("id"),
				Operator: r3.OperatorEq,
				Value:    123,
			},
			validate: func(t *testing.T, result *r3json.JSONFilter) {
				assert.Equal(t, "id", string(result.Field))
				assert.Equal(t, r3json.OperatorEq, result.Op)
				assert.Equal(t, 123, result.Value)
			},
		},
		{
			name: "filter with AND groups",
			input: &r3.FilterSpec{
				And: r3.Filters{
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("age"),
						Operator: r3.OperatorGte,
						Value:    18,
					},
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("status"),
						Operator: r3.OperatorEq,
						Value:    "active",
					},
				},
			},
			validate: func(t *testing.T, result *r3json.JSONFilter) {
				require.Len(t, result.And, 2)
				assert.Equal(t, "age", string(result.And[0].Field))
				assert.Equal(t, "status", string(result.And[1].Field))
			},
		},
		{
			name:        "nil filter spec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3json.FilterToJSON(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestSortToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    *r3.SortSpec
		validate func(t *testing.T, result *r3json.JSONSort)
		isNil    bool
	}{
		{
			name: "valid sort spec",
			input: &r3.SortSpec{
				Column:    r3.NewFieldSpec("name"),
				Direction: r3.SortDirectionAsc,
			},
			validate: func(t *testing.T, result *r3json.JSONSort) {
				assert.Equal(t, "name", string(result.Field))
				assert.Equal(t, r3json.JSONSortDirectionAsc, result.Direction)
			},
		},
		{
			name:  "nil sort spec",
			input: nil,
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3json.SortToJSON(tt.input)
			if tt.isNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestPaginationToJSON(t *testing.T) {
	p := r3.NewPaginationSpec(2, 25)
	result := r3json.PaginationToJSON(p)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.PageNum)
	assert.Equal(t, 25, result.PageSize)
}

func TestJSONToPagination(t *testing.T) {
	jp := &r3json.JSONPagination{PageNum: 3, PageSize: 10}
	result, err := r3json.JSONToPagination(jp)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.GetPageNum())
	assert.Equal(t, 10, result.GetPageSize())
}
