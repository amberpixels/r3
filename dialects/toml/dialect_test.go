package r3toml_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3toml "github.com/amberpixels/r3/dialects/toml"
	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldToTOML(t *testing.T) {
	tests := []struct {
		name     string
		input    *r3.FieldSpec
		expected r3toml.TOMLField
	}{
		{"valid field", r3.NewFieldSpec("name"), "name"},
		{"nil field", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3toml.FieldToTOML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTOMLToField(t *testing.T) {
	result := r3toml.TOMLToField("name")
	assert.Equal(t, "name", result.String())
}

func TestFilterToTOML_RelationFilterRejected(t *testing.T) {
	// A relationship ("has") filter has no serialized form; encoding must error
	// rather than silently drop the relation into a match-all filter.
	_, err := r3toml.FilterToTOML(r3.Has("Squads", r3.In("id", []int64{1, 3})))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Squads")
}

func TestFilterToTOML(t *testing.T) {
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
			result, err := r3toml.FilterToTOML(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestFilterToTOML_NilValue(t *testing.T) {
	// TOML does not support null values: a value-carrying operator with a nil
	// value must be rejected.
	filter := &r3.FilterSpec{
		Field:    r3.NewFieldSpec("status"),
		Operator: r3.OperatorEq,
		Value:    nil,
	}

	_, err := r3toml.FilterToTOML(filter)
	require.Error(t, err)
	require.ErrorIs(t, err, r3toml.ErrTOMLNullValue)
	assert.True(t, r3toml.IsErrDialectorFailure(err))
}

func TestFilterToTOML_ExistsHasNoValue(t *testing.T) {
	// "exists" carries no value and must NOT be rejected by the null guard —
	// it has to round-trip like every other serialization dialect.
	filter := &r3.FilterSpec{
		Field:    r3.NewFieldSpec("status"),
		Operator: r3.OperatorExists,
		Value:    nil,
	}

	tf, err := r3toml.FilterToTOML(filter)
	require.NoError(t, err)
	assert.Equal(t, "status", tf.Field)
	assert.Equal(t, "exists", tf.Operator)
	assert.Nil(t, tf.Value)

	// Round-trip back to a FilterSpec.
	back, err := tf.ToFilterSpec()
	require.NoError(t, err)
	assert.Equal(t, r3.OperatorExists, back.Operator)
	assert.Equal(t, "status", back.Field.String())
	assert.Nil(t, back.Value)
}

func TestFilterToTOML_SimpleFilter(t *testing.T) {
	filter := r3.F(r3.NewFieldSpec("country"), "United States")
	tf, err := r3toml.FilterToTOML(filter)
	require.NoError(t, err)

	assert.Equal(t, "country", tf.Field)
	assert.Equal(t, "eq", tf.Operator)
	assert.Equal(t, "United States", tf.Value)
}

func TestTOMLToFilter(t *testing.T) {
	tf := &r3toml.TOMLFilter{
		Field:    "country",
		Operator: "eq",
		Value:    "United States",
	}

	f, err := r3toml.TOMLToFilter(tf)
	require.NoError(t, err)
	assert.Equal(t, "country", f.Field.String())
	assert.Equal(t, r3.OperatorEq, f.Operator)
	assert.Equal(t, "United States", f.Value)
}

func TestTOMLToFilters(t *testing.T) {
	tfs := r3toml.TOMLFilters{
		{Field: "country", Operator: "eq", Value: "US"},
		{Field: "age", Operator: "gt", Value: int64(18)},
	}

	filters, err := r3toml.TOMLToFilters(tfs)
	require.NoError(t, err)
	assert.Len(t, filters, 2)
	assert.Equal(t, "country", filters[0].Field.String())
	assert.Equal(t, r3.OperatorEq, filters[0].Operator)
	assert.Equal(t, "age", filters[1].Field.String())
	assert.Equal(t, r3.OperatorGt, filters[1].Operator)
}

func TestSortToTOML(t *testing.T) {
	tests := []struct {
		name  string
		input *r3.SortSpec
		want  *r3toml.TOMLSort
	}{
		{
			name:  "ascending sort",
			input: r3.NewSortSpec(r3.NewFieldSpec("created_at"), r3.SortDirectionAsc),
			want:  &r3toml.TOMLSort{Field: "created_at", Direction: "asc"},
		},
		{
			name:  "descending sort with nulls last",
			input: r3.NewSortSpec(r3.NewFieldSpec("updated_at"), r3.SortDirectionDesc, r3.NullsPositionLast),
			want:  &r3toml.TOMLSort{Field: "updated_at", Direction: "desc", NullsPosition: "last"},
		},
		{
			name:  "nil sort",
			input: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3toml.SortToTOML(tt.input)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestTOMLToSort(t *testing.T) {
	ts := &r3toml.TOMLSort{
		Field:         "created_at",
		Direction:     "desc",
		NullsPosition: "last",
	}

	s, err := r3toml.TOMLToSort(ts)
	require.NoError(t, err)
	assert.Equal(t, "created_at", s.Column.String())
	assert.Equal(t, r3.SortDirectionDesc, s.Direction)
	assert.Equal(t, r3.NullsPositionLast, s.NullsPosition)
}

func TestPaginationToTOML(t *testing.T) {
	p := r3.NewPaginationSpec(2, 25)
	tp := r3toml.PaginationToTOML(p)
	assert.Equal(t, 2, tp.PageNum)
	assert.Equal(t, 25, tp.PageSize)
}

func TestTOMLToPagination(t *testing.T) {
	tp := &r3toml.TOMLPagination{PageNum: 3, PageSize: 50}
	p, err := r3toml.TOMLToPagination(tp)
	require.NoError(t, err)
	assert.Equal(t, 3, p.GetPageNum())
	assert.Equal(t, 50, p.GetPageSize())
}

func TestTOMLToPagination_Empty(t *testing.T) {
	tp := &r3toml.TOMLPagination{}
	p, err := r3toml.TOMLToPagination(tp)
	require.NoError(t, err)
	assert.False(t, p.IsPaginated())
}

func TestTOMLFilter_RoundTrip(t *testing.T) {
	original := r3.Fop(r3.NewFieldSpec("popularity"), r3.OperatorGte, 50)

	tf, err := r3toml.FilterToTOML(original)
	require.NoError(t, err)

	restored, err := r3toml.TOMLToFilter(tf)
	require.NoError(t, err)

	assert.Equal(t, original.Field.String(), restored.Field.String())
	assert.Equal(t, original.Operator, restored.Operator)
	assert.Equal(t, original.Value, restored.Value)
}

func TestTOMLSort_RoundTrip(t *testing.T) {
	original := r3.NewSortSpec(r3.NewFieldSpec("created_at"), r3.SortDirectionDesc, r3.NullsPositionLast)

	ts := r3toml.SortToTOML(original)
	restored, err := r3toml.TOMLToSort(ts)
	require.NoError(t, err)

	assert.Equal(t, original.Column.String(), restored.Column.String())
	assert.Equal(t, original.Direction, restored.Direction)
	assert.Equal(t, original.NullsPosition, restored.NullsPosition)
}

func TestTOMLPagination_RoundTrip(t *testing.T) {
	original := r3.NewPaginationSpec(5, 30)

	tp := r3toml.PaginationToTOML(original)
	restored, err := r3toml.TOMLToPagination(tp)
	require.NoError(t, err)

	assert.Equal(t, original.GetPageNum(), restored.GetPageNum())
	assert.Equal(t, original.GetPageSize(), restored.GetPageSize())
}

func TestTOMLFilter_MarshalUnmarshal(t *testing.T) {
	tf := &r3toml.TOMLFilter{
		Field:    "country",
		Operator: "eq",
		Value:    "United States",
	}

	data, err := toml.Marshal(tf)
	require.NoError(t, err)

	var restored r3toml.TOMLFilter
	err = toml.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, tf.Field, restored.Field)
	assert.Equal(t, tf.Operator, restored.Operator)
	assert.Equal(t, tf.Value, restored.Value)
}

func TestTOMLSort_MarshalUnmarshal(t *testing.T) {
	ts := &r3toml.TOMLSort{
		Field:         "created_at",
		Direction:     "desc",
		NullsPosition: "last",
	}

	data, err := toml.Marshal(ts)
	require.NoError(t, err)

	var restored r3toml.TOMLSort
	err = toml.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, ts.Field, restored.Field)
	assert.Equal(t, ts.Direction, restored.Direction)
	assert.Equal(t, ts.NullsPosition, restored.NullsPosition)
}

func TestTOMLPagination_MarshalUnmarshal(t *testing.T) {
	tp := &r3toml.TOMLPagination{
		PageNum:  1,
		PageSize: 20,
	}

	data, err := toml.Marshal(tp)
	require.NoError(t, err)

	var restored r3toml.TOMLPagination
	err = toml.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, tp.PageNum, restored.PageNum)
	assert.Equal(t, tp.PageSize, restored.PageSize)
}

func TestTOMLFilter_ComplexTOMLUnmarshal(t *testing.T) {
	// Test the exact TOML format from the plan
	tomlData := `
[[filters]]
field = "country"
operator = "eq"
value = "United States"

[[filters]]
field = "popularity"
operator = "gt"
value = 50

[[sorts]]
field = "created_at"
direction = "desc"

[pagination]
page_num = 1
page_size = 20
`

	var doc struct {
		Filters    r3toml.TOMLFilters     `toml:"filters"`
		Sorts      []*r3toml.TOMLSort     `toml:"sorts"`
		Pagination *r3toml.TOMLPagination `toml:"pagination"`
	}

	err := toml.Unmarshal([]byte(tomlData), &doc)
	require.NoError(t, err)

	// Verify filters
	require.Len(t, doc.Filters, 2)
	assert.Equal(t, "country", doc.Filters[0].Field)
	assert.Equal(t, "eq", doc.Filters[0].Operator)
	assert.Equal(t, "United States", doc.Filters[0].Value)
	assert.Equal(t, "popularity", doc.Filters[1].Field)
	assert.Equal(t, "gt", doc.Filters[1].Operator)
	assert.Equal(t, int64(50), doc.Filters[1].Value)

	// Convert to r3 types
	filters, err := r3toml.TOMLToFilters(doc.Filters)
	require.NoError(t, err)
	assert.Len(t, filters, 2)
	assert.Equal(t, r3.OperatorEq, filters[0].Operator)
	assert.Equal(t, r3.OperatorGt, filters[1].Operator)

	// Verify sorts
	require.Len(t, doc.Sorts, 1)
	sort, err := r3toml.TOMLToSort(doc.Sorts[0])
	require.NoError(t, err)
	assert.Equal(t, "created_at", sort.Column.String())
	assert.Equal(t, r3.SortDirectionDesc, sort.Direction)

	// Verify pagination
	require.NotNil(t, doc.Pagination)
	pagination, err := r3toml.TOMLToPagination(doc.Pagination)
	require.NoError(t, err)
	assert.Equal(t, 1, pagination.GetPageNum())
	assert.Equal(t, 20, pagination.GetPageSize())
}

func TestTOMLFilter_AllOperators(t *testing.T) {
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
			tf := &r3toml.TOMLFilter{
				Field:    "test_field",
				Operator: tt.name,
				Value:    "test_value",
			}

			f, err := r3toml.TOMLToFilter(tf)
			require.NoError(t, err)
			assert.Equal(t, tt.expect, f.Operator)
		})
	}
}

func TestTOMLFilter_InvalidOperator(t *testing.T) {
	tf := &r3toml.TOMLFilter{
		Field:    "test_field",
		Operator: "invalid_op",
		Value:    "test_value",
	}

	_, err := r3toml.TOMLToFilter(tf)
	require.Error(t, err)
	assert.True(t, r3toml.IsErrDialectorFailure(err))
}

func TestTOMLFilter_EmptyField(t *testing.T) {
	tf := &r3toml.TOMLFilter{
		Field:    "",
		Operator: "eq",
		Value:    "test_value",
	}

	_, err := r3toml.TOMLToFilter(tf)
	require.Error(t, err)
}

func TestTOMLSort_EmptyField(t *testing.T) {
	ts := &r3toml.TOMLSort{
		Field:     "",
		Direction: "asc",
	}

	_, err := r3toml.TOMLToSort(ts)
	require.Error(t, err)
}
