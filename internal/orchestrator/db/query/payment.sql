-- name: CreatePayment :one
INSERT INTO payment (id, end_to_end_id, source_bic, destination_bic,
                     source_account, dest_account, amount, currency, status,
                     uetr, instr_id, charge_bearer, sttlm_dt,
                     debtor_name, creditor_name, purpose_code, remittance_info)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING *;

-- name: UpdatePaymentStatus :exec
UPDATE payment SET status = $1, updated_at = now() WHERE id = $2;

-- name: UpdatePaymentStatusReturning :one
UPDATE payment SET status = $1, updated_at = now() WHERE id = $2
RETURNING status;

-- name: MarkPaymentReserved :exec
UPDATE payment
SET status = $1, reserved_at = $2, expires_at = $3, updated_at = now()
WHERE id = $4;

-- name: GetPaymentByID :one
SELECT * FROM payment WHERE id = $1;

-- name: GetPaymentByEndToEndID :one
SELECT * FROM payment WHERE end_to_end_id = $1;

-- name: FindExpiredReservations :many
SELECT id, source_account, amount, reserved_at, expires_at
FROM payment
WHERE status = 'RESERVED' AND expires_at < $1
FOR UPDATE SKIP LOCKED;

-- name: GetPaymentEventFields :one
SELECT id, end_to_end_id, source_bic, destination_bic, amount, currency
FROM payment WHERE id = $1;
