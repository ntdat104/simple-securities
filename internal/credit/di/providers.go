package di

import (
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"

	"simple-securities/internal/credit/application/service"
	"simple-securities/internal/credit/domain/messaging"
	"simple-securities/internal/credit/domain/repo"
	"simple-securities/internal/credit/infras/idem"
	infralock "simple-securities/internal/credit/infras/lock"
	inframsg "simple-securities/internal/credit/infras/messaging"
	infrarepo "simple-securities/internal/credit/infras/repo"
	"simple-securities/pkg/db/txmanager"
	"simple-securities/pkg/kafka"
)

// --- Repos ---

func NewCreditWalletRepo(db *sqlx.DB) repo.ICreditWalletRepo {
	return infrarepo.NewCreditWalletRepo(db)
}

func NewCreditTransactionRepo(db *sqlx.DB) repo.ICreditTransactionRepo {
	return infrarepo.NewCreditTransactionRepo(db)
}

func NewCreditReservationRepo(db *sqlx.DB) repo.ICreditReservationRepo {
	return infrarepo.NewCreditReservationRepo(db)
}

// --- Infrastructure ---

func NewDistributedLock(client *redis.Client) service.DistributedLock {
	return infralock.NewRedisDistributedLock(client)
}

func NewIdempotencyStore(client *redis.Client) service.IdempotencyStore {
	return idem.NewRedisIdempotencyStore(client)
}

func NewKafkaEventPublisher(producer *kafka.Producer) messaging.IEventPublisher {
	return inframsg.NewKafkaEventPublisher(producer)
}

// --- Services ---

func NewTopUpSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	txManager txmanager.TxManager,
	lock service.DistributedLock,
	idemStore service.IdempotencyStore,
	publisher messaging.IEventPublisher,
) service.TopUpSvc {
	return service.NewTopUpSvc(walletRepo, txRepo, txManager, lock, idemStore, publisher)
}

func NewReserveCreditsSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	reservationRepo repo.ICreditReservationRepo,
	reservationItemRepo repo.ICreditReservationItemRepo,
	txManager txmanager.TxManager,
	lock service.DistributedLock,
	idemStore service.IdempotencyStore,
	publisher messaging.IEventPublisher,
) service.ReserveCreditsSvc {
	return service.NewReserveCreditsSvc(walletRepo, txRepo, reservationRepo, reservationItemRepo, txManager, lock, idemStore, publisher)
}

func NewCommitReservationSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	reservationRepo repo.ICreditReservationRepo,
	reservationItemRepo repo.ICreditReservationItemRepo,
	txManager txmanager.TxManager,
	lock service.DistributedLock,
	idemStore service.IdempotencyStore,
	publisher messaging.IEventPublisher,
) service.CommitReservationSvc {
	return service.NewCommitReservationSvc(walletRepo, txRepo, reservationRepo, reservationItemRepo, txManager, lock, idemStore, publisher)
}

func NewRollbackReservationSvc(
	walletRepo repo.ICreditWalletRepo,
	txRepo repo.ICreditTransactionRepo,
	reservationRepo repo.ICreditReservationRepo,
	reservationItemRepo repo.ICreditReservationItemRepo,
	txManager txmanager.TxManager,
	lock service.DistributedLock,
	publisher messaging.IEventPublisher,
) service.RollbackReservationSvc {
	return service.NewRollbackReservationSvc(walletRepo, txRepo, reservationRepo, reservationItemRepo, txManager, lock, publisher)
}

func NewGetCreditBalanceSvc(walletRepo repo.ICreditWalletRepo) service.GetCreditBalanceSvc {
	return service.NewGetCreditBalanceSvc(walletRepo)
}
