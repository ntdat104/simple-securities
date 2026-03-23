package enum

type ReservationStatus string

const (
	ReservationPending    ReservationStatus = "PENDING"
	ReservationCommitted  ReservationStatus = "COMMITTED"
	ReservationRolledBack ReservationStatus = "ROLLED_BACK"
)
