-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES (
    $1, NOW(), NOW(), $2, NOW() + INTERVAL '60 days', NULL
)
RETURNING *;

-- name: GetUserFromRefreshToken :one
SELECT user_id
FROM refresh_tokens
WHERE token = $1;

-- name: GetRefreshToken :one
SELECT token, created_at, updated_at, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token = $1;