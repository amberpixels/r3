// Package r3when is the bridge between the years schedule vocabulary and r3
// filters. It compiles recurring weekly wall-clock patterns
// (schedule.WeekPatterns) into r3.Filters built from the vocabulary-free
// OperatorWeekdayIn and OperatorTimeOfDayBetween primitives, and resolves human
// terms ("weekends", "mornings") into those patterns via years.
//
// It is a dialect in the same sense as the URL or JSON dialects: it translates
// an external, human representation of a filter into the r3 query model. Keeping
// it here - not in the r3 core - is what lets the core stay free of the years
// dependency and free of any time vocabulary. r3 knows how to compare time
// components; only this dialect knows that "weekends" means Saturday and Sunday.
//
// The compiled filters use nothing but root filter vocabulary, so they serialize
// through every r3 dialect and run on any engine that lowers the two operators
// (the file engine and Mongo today; SQL once the flavor hook lands - see
// docs/plan-when-filters.md).
package r3when
