-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(), NOW(), NOW(), $1, $2
)
RETURNING *;

-- name: GetChirpsInOrder :many
SELECT id, created_at, updated_at, body, user_id
FROM chirps
ORDER BY created_at ASC;