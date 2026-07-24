package r3yaml_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/amberpixels/r3"
	r3yaml "github.com/amberpixels/r3/dialects/yaml"
)

func TestFieldToYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    *r3.FieldSpec
		expected r3yaml.YAMLField
	}{
		{"valid field", r3.NewFieldSpec("name"), "name"},
		{"nil field", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3yaml.FieldToYAML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestYAMLToField(t *testing.T) {
	result := r3yaml.YAMLToField("name")
	assert.Equal(t, "name", result.String())
}

func TestFilterToYAML_RelationFilterRejected(t *testing.T) {
	// A relationship ("has") filter has no serialized form; encoding must error
	// rather than silently drop the relation into a match-all filter.
	_, err := r3yaml.FilterToYAML(r3.Has("Squads", r3.In("id", []int64{1, 3})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Squads")
}

func TestFilterToYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   *r3.FilterSpec
		wantErr bool
	}{
		{
			name:    "simple filter",
			input:   r3.F(r3.NewFieldSpec("country"), "US"),
			wantErr: false,
		},
		{
			name: "filter with AND group",
			input: r3.And(
				r3.F(r3.NewFieldSpec("country"), "US"),
				r3.Fop(r3.NewFieldSpec("age"), r3.OperatorGt, 18),
			),
			wantErr: false,
		},
		{
			name:    "nil filter",
			input:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3yaml.FilterToYAML(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestFilterToYAML_SimpleFilter(t *testing.T) {
	filter := r3.F(r3.NewFieldSpec("country"), "United States")
	yf, err := r3yaml.FilterToYAML(filter)
	require.NoError(t, err)

	assert.Equal(t, "country", yf.Field)
	assert.Equal(t, "eq", yf.Operator)
	assert.Equal(t, "United States", yf.Value)
}

func TestYAMLToFilter(t *testing.T) {
	yf := &r3yaml.YAMLFilter{
		Field:    "country",
		Operator: "eq",
		Value:    "United States",
	}

	f, err := r3yaml.YAMLToFilter(yf)
	require.NoError(t, err)
	assert.Equal(t, "country", f.Field.String())
	assert.Equal(t, r3.OperatorEq, f.Operator)
	assert.Equal(t, "United States", f.Value)
}

func TestYAMLToFilters(t *testing.T) {
	yfs := r3yaml.YAMLFilters{
		{Field: "country", Operator: "eq", Value: "US"},
		{Field: "age", Operator: "gt", Value: 18},
	}

	filters, err := r3yaml.YAMLToFilters(yfs)
	require.NoError(t, err)
	assert.Len(t, filters, 2)
	assert.Equal(t, "country", filters[0].Field.String())
	assert.Equal(t, r3.OperatorEq, filters[0].Operator)
	assert.Equal(t, "age", filters[1].Field.String())
	assert.Equal(t, r3.OperatorGt, filters[1].Operator)
}

func TestSortToYAML(t *testing.T) {
	tests := []struct {
		name  string
		input *r3.SortSpec
		want  *r3yaml.YAMLSort
	}{
		{
			name:  "ascending sort",
			input: r3.NewSortSpec(r3.NewFieldSpec("created_at"), r3.SortDirectionAsc),
			want:  &r3yaml.YAMLSort{Field: "created_at", Direction: "asc"},
		},
		{
			name:  "descending sort with nulls last",
			input: r3.NewSortSpec(r3.NewFieldSpec("updated_at"), r3.SortDirectionDesc, r3.NullsPositionLast),
			want:  &r3yaml.YAMLSort{Field: "updated_at", Direction: "desc", NullsPosition: "last"},
		},
		{
			name:  "nil sort",
			input: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3yaml.SortToYAML(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestYAMLToSort(t *testing.T) {
	ys := &r3yaml.YAMLSort{
		Field:         "created_at",
		Direction:     "desc",
		NullsPosition: "last",
	}

	s, err := r3yaml.YAMLToSort(ys)
	require.NoError(t, err)
	assert.Equal(t, "created_at", s.Column.String())
	assert.Equal(t, r3.SortDirectionDesc, s.Direction)
	assert.Equal(t, r3.NullsPositionLast, s.NullsPosition)
}

func TestPaginationToYAML(t *testing.T) {
	p := r3.NewPaginationSpec(2, 25)
	yp := r3yaml.PaginationToYAML(p)
	assert.Equal(t, 2, yp.PageNum)
	assert.Equal(t, 25, yp.PageSize)
}

func TestYAMLToPagination(t *testing.T) {
	yp := &r3yaml.YAMLPagination{PageNum: 3, PageSize: 50}
	p, err := r3yaml.YAMLToPagination(yp)
	require.NoError(t, err)
	assert.Equal(t, 3, p.GetPageNum())
	assert.Equal(t, 50, p.GetPageSize())
}

func TestYAMLToPagination_Nil(t *testing.T) {
	yp := &r3yaml.YAMLPagination{}
	p, err := r3yaml.YAMLToPagination(yp)
	require.NoError(t, err)
	assert.False(t, p.IsPaginated())
}

func TestYAMLFilter_RoundTrip(t *testing.T) {
	// Create an r3 filter, convert to YAML, then back
	original := r3.Fop(r3.NewFieldSpec("popularity"), r3.OperatorGte, 50)

	yf, err := r3yaml.FilterToYAML(original)
	require.NoError(t, err)

	restored, err := r3yaml.YAMLToFilter(yf)
	require.NoError(t, err)

	assert.Equal(t, original.Field.String(), restored.Field.String())
	assert.Equal(t, original.Operator, restored.Operator)
	assert.Equal(t, original.Value, restored.Value)
}

func TestYAMLSort_RoundTrip(t *testing.T) {
	original := r3.NewSortSpec(r3.NewFieldSpec("created_at"), r3.SortDirectionDesc, r3.NullsPositionLast)

	ys := r3yaml.SortToYAML(original)
	restored, err := r3yaml.YAMLToSort(ys)
	require.NoError(t, err)

	assert.Equal(t, original.Column.String(), restored.Column.String())
	assert.Equal(t, original.Direction, restored.Direction)
	assert.Equal(t, original.NullsPosition, restored.NullsPosition)
}

func TestYAMLPagination_RoundTrip(t *testing.T) {
	original := r3.NewPaginationSpec(5, 30)

	yp := r3yaml.PaginationToYAML(original)
	restored, err := r3yaml.YAMLToPagination(yp)
	require.NoError(t, err)

	assert.Equal(t, original.GetPageNum(), restored.GetPageNum())
	assert.Equal(t, original.GetPageSize(), restored.GetPageSize())
}

func TestYAMLFilter_MarshalUnmarshal(t *testing.T) {
	yf := &r3yaml.YAMLFilter{
		Field:    "country",
		Operator: "eq",
		Value:    "United States",
	}

	data, err := yaml.Marshal(yf)
	require.NoError(t, err)

	var restored r3yaml.YAMLFilter
	err = yaml.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, yf.Field, restored.Field)
	assert.Equal(t, yf.Operator, restored.Operator)
	assert.Equal(t, yf.Value, restored.Value)
}

func TestYAMLSort_MarshalUnmarshal(t *testing.T) {
	ys := &r3yaml.YAMLSort{
		Field:         "created_at",
		Direction:     "desc",
		NullsPosition: "last",
	}

	data, err := yaml.Marshal(ys)
	require.NoError(t, err)

	var restored r3yaml.YAMLSort
	err = yaml.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, ys.Field, restored.Field)
	assert.Equal(t, ys.Direction, restored.Direction)
	assert.Equal(t, ys.NullsPosition, restored.NullsPosition)
}

func TestYAMLPagination_MarshalUnmarshal(t *testing.T) {
	yp := &r3yaml.YAMLPagination{
		PageNum:  1,
		PageSize: 20,
	}

	data, err := yaml.Marshal(yp)
	require.NoError(t, err)

	var restored r3yaml.YAMLPagination
	err = yaml.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, yp.PageNum, restored.PageNum)
	assert.Equal(t, yp.PageSize, restored.PageSize)
}

func TestYAMLFilter_ComplexYAMLUnmarshal(t *testing.T) {
	// Test unmarshaling the exact YAML format from the plan
	yamlData := `
filters:
  - field: country
    operator: eq
    value: "United States"
  - field: popularity
    operator: gt
    value: 50
sorts:
  - field: created_at
    direction: desc
    nulls_position: last
pagination:
  page_num: 1
  page_size: 20
`

	var doc struct {
		Filters    r3yaml.YAMLFilters     `yaml:"filters"`
		Sorts      []*r3yaml.YAMLSort     `yaml:"sorts"`
		Pagination *r3yaml.YAMLPagination `yaml:"pagination"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &doc)
	require.NoError(t, err)

	// Verify filters
	require.Len(t, doc.Filters, 2)
	assert.Equal(t, "country", doc.Filters[0].Field)
	assert.Equal(t, "eq", doc.Filters[0].Operator)
	assert.Equal(t, "United States", doc.Filters[0].Value)
	assert.Equal(t, "popularity", doc.Filters[1].Field)
	assert.Equal(t, "gt", doc.Filters[1].Operator)
	assert.Equal(t, 50, doc.Filters[1].Value)

	// Convert to r3 types
	filters, err := r3yaml.YAMLToFilters(doc.Filters)
	require.NoError(t, err)
	assert.Len(t, filters, 2)
	assert.Equal(t, r3.OperatorEq, filters[0].Operator)
	assert.Equal(t, r3.OperatorGt, filters[1].Operator)

	// Verify sorts
	require.Len(t, doc.Sorts, 1)
	sort, err := r3yaml.YAMLToSort(doc.Sorts[0])
	require.NoError(t, err)
	assert.Equal(t, "created_at", sort.Column.String())
	assert.Equal(t, r3.SortDirectionDesc, sort.Direction)
	assert.Equal(t, r3.NullsPositionLast, sort.NullsPosition)

	// Verify pagination
	require.NotNil(t, doc.Pagination)
	pagination, err := r3yaml.YAMLToPagination(doc.Pagination)
	require.NoError(t, err)
	assert.Equal(t, 1, pagination.GetPageNum())
	assert.Equal(t, 20, pagination.GetPageSize())
}

func TestYAMLFilter_NestedAndOr(t *testing.T) {
	yamlData := `
and:
  - field: status
    operator: eq
    value: active
  - or:
      - field: role
        operator: eq
        value: admin
      - field: role
        operator: eq
        value: moderator
`
	var yf r3yaml.YAMLFilter
	err := yaml.Unmarshal([]byte(yamlData), &yf)
	require.NoError(t, err)

	f, err := r3yaml.YAMLToFilter(&yf)
	require.NoError(t, err)

	// Root should be an AND group
	assert.Len(t, f.And, 2)
	assert.Equal(t, "status", f.And[0].Field.String())
	assert.Equal(t, r3.OperatorEq, f.And[0].Operator)

	// Second AND child should be an OR group
	assert.Len(t, f.And[1].Or, 2)
	assert.Equal(t, "role", f.And[1].Or[0].Field.String())
	assert.Equal(t, "admin", f.And[1].Or[0].Value)
	assert.Equal(t, "role", f.And[1].Or[1].Field.String())
	assert.Equal(t, "moderator", f.And[1].Or[1].Value)
}

func TestYAMLFilter_AllOperators(t *testing.T) {
	operators := []struct {
		name   string
		expect r3.FilterOperatorSpec
	}{
		{"eq", r3.OperatorEq},
		{"ne", r3.OperatorNe},
		{"exists", r3.OperatorExists},
		{"gt", r3.OperatorGt},
		{"gte", r3.OperatorGte},
		{"lt", r3.OperatorLt},
		{"lte", r3.OperatorLte},
		{"between", r3.OperatorBetween},
		{"between_exc", r3.OperatorBetweenEx},
		{"between_exc_inc", r3.OperatorBetweenExInc},
		{"between_inc_exc", r3.OperatorBetweenIncEx},
		{"in", r3.OperatorIn},
		{"nin", r3.OperatorNotIn},
		{"like", r3.OperatorLike},
		{"notlike", r3.OperatorNotLike},
		{"ilike", r3.OperatorILike},
	}

	for _, tt := range operators {
		t.Run(tt.name, func(t *testing.T) {
			yf := &r3yaml.YAMLFilter{
				Field:    "test_field",
				Operator: tt.name,
				Value:    "test_value",
			}

			f, err := r3yaml.YAMLToFilter(yf)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, f.Operator)
		})
	}
}

func TestYAMLFilter_InvalidOperator(t *testing.T) {
	yf := &r3yaml.YAMLFilter{
		Field:    "test_field",
		Operator: "invalid_op",
		Value:    "test_value",
	}

	_, err := r3yaml.YAMLToFilter(yf)
	require.Error(t, err)
	assert.True(t, r3yaml.IsErrDialectorFailure(err))
}

func TestYAMLFilter_EmptyField(t *testing.T) {
	yf := &r3yaml.YAMLFilter{
		Field:    "",
		Operator: "eq",
		Value:    "test_value",
	}

	_, err := r3yaml.YAMLToFilter(yf)
	require.Error(t, err)
}

func TestYAMLSort_EmptyField(t *testing.T) {
	ys := &r3yaml.YAMLSort{
		Field:     "",
		Direction: "asc",
	}

	_, err := r3yaml.YAMLToSort(ys)
	require.Error(t, err)
}
