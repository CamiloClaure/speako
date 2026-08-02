// Package ordering owns the order aggregate and its state machine
// (pending -> paid|failed|expired, paid -> refunded) and the CDC art. 49
// seven-day withdrawal window. Stdlib imports only.
package ordering
