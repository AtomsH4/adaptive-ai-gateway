package ports

// OwnedProcesses is consumed by platform measurement, never by HTTP or policy.
// It supplies only live child PIDs owned by this runtime; it grants no kill right.
type OwnedProcesses interface{ OwnedPIDs() []int }
