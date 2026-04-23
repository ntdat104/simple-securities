package enum

type TxType string

const (
	TxTypeTopUp    TxType = "TOP_UP"
	TxTypeReserve  TxType = "RESERVE"
	TxTypeCommit   TxType = "COMMIT"
	TxTypeRollback TxType = "ROLLBACK"
	TxTypeExpire   TxType = "EXPIRE"
)
