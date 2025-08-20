# Chirpy
This project was made as practice during the [Learn HTTP Servers in Go](https://www.boot.dev/courses/learn-http-servers-golang) course on [Boot.dev](https://boot.dev).

Chirpy is a social network similar to Twitter, and this repo is part of the backend of Chirpy.


# API Endpoints
## Chirpy's file server:
The file server can be accessed on the `/app` endpoint. It currently only includes a simple `index.html` file welcoming you to Chirpy, and Chirpy's logo at `/assets/logo.png`.

## Readiness endpoint:
Chirpy's readiness endpoint is `GET /api/healthz`, it should always respond with a 200 status code as long as the server is running.

## Admin endpoints:
There are currently 2 admin endpoints:
 - `GET /admin/metrics`, this endpoint responds with code 200 and a simple HTML file that includes a counter of how many times the file server has been accessed.
 - `POST /admin/reset`, this endpoint resets the file server hits counter to 0 and clears the database. Responds with 200 on success.
   - The reset endpoint only works if the `PLATFORM` environment variable is set to `dev`, otherwise responds with a 403 code.

## User operations:
### POST /api/users
The first thing a user does is register, or in other words, create a user in the system. This can be done using this endpoint. It requires the request body to be of the form:
```json
{
  "email": "{email}",
  "password": "{password}"
}
```
Where `{email}` and `{password}` are the new user's desired email and password, respectively.

On success, it responds with a 201 code and the created user's info, for example:
```json
{
    "id": "781fa276-8d09-4999-a720-037c51640f19",
    "created_at": "2025-08-20T17:23:41.65783Z",
    "updated_at": "2025-08-20T17:23:41.65783Z",
    "email": "testmail@mail.com",
    "is_chirpy_red": false
}
```
 - `id` is the new user's UUID.
 - `created_at` is the user's creation time.
 - `updated_at` is the time of the most recent update to this user.
 - `email` is this user's email address.
 - `is_chirpy_red` says whether this user is a Chirpy Red member or not.

If the email is already in use by another user, it responds with a 422 code and the following body:
```json
{
    "error": "Email is already used by another user"
}
```

### POST /api/login
After creating a new user, we need to log in to get access to other user operations, that's what this endpoint is for.
To log in, simply send a request with a body of the following form:
```json
{
    "email": "{email}",
    "password": "{password}"
}
```
Where `{email}` and `{password}` are the user's email and password respectively.

If there is a user with the given email and password, it responds with a code of 200, and a body similar to the following example:
```json
{
    "id": "781fa276-8d09-4999-a720-037c51640f19",
    "created_at": "2025-08-20T17:23:41.65783Z",
    "updated_at": "2025-08-20T17:23:41.65783Z",
    "email": "testmail@mail.com",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjaGlycHkiLCJzdWIiOiI3ODFmYTI3Ni04ZDA5LTQ5OTktYTcyMC0wMzdjNTE2NDBmMTkiLCJleHAiOjE3NTU3MDM2NTUsImlhdCI6MTc1NTcwMDA1NX0.KDnqukN3Z2SMrcbp15cyVzU3Rl1UDWAcVI8WN8QmeHo",
    "refresh_token": "14af12741a37b8f590d746b7df47500e4444d74286b4fc6c0d1a5a8bf510a057",
    "is_chirpy_red": false
}
```
 - `id` is this user's UUID.
 - `created_at` is this user's creation time.
 - `updated_at` is the time of the most recent update to this user.
 - `email` is this user's email address.
 - `token` is the access token of this user for the current session, it expires after an hour and can be refreshed using the next endpoint.
 - `refresh_token` is the refresh token of this user for the current session, it expires after 60 days and can be used to refresh access tokens.
 - `is_chirpy_red` says whether this user is a Chirpy Red member or not.

If either the email doesn't exist, or the password is incorrect, a response code of 401 is given with the following body:
```json
{
    "error": "incorrect email or password"
}
```

### POST /api/refresh
This endpoint is used to generate new access tokens for a user without having to log in every hour when the previous access token expires.

The request must have an `Authorization` header with the value `Bearer {refreshToken}`, where `{refreshToken}` is the user's refresh token.

If the refresh token is valid (hasn't expired or wasn't revoked), a 200 response code is given with a body of the following format:
```json
{
    "token": "{accessToken}"
}
```
Where `{accessToken}` is a new token for the user. 

If the used refresh token is invalid, expired, or was revoked, a 401 response will be given.

### POST /api/revoke
This endpoint is used to revoke a refresh token, so it cannot be used to generate any new access tokens to a user's account.

The request must have an `Authorization` header with the value `Bearer {refreshToken}`, where `{refreshToken}` is the refresh token to revoke.

If the refresh token is valid (hasn't expired or wasn't revoked), a 204 response is given, otherwise the response code will be 401.

### PUT /api/users
This endpoint updates a user's email and password.

The request must have an `Authorization` header with the value `Bearer {accessToken}`, where `{accessToken}` is the user's access token, and a body of the form:
```json
{
  "email": "{newEmail}",
  "password": "{newPassword}"
}
```
Where `{newEmail}` and `{newPassword}` are the new email and password respectively. Alternatively, if you want to keep one of them unchanged, you can pass the old value in the request body.

If the access token is valid, the email and password of the user will be updated, and a response of code 200 will be given with a body similar to this example:
```json
{
    "id": "781fa276-8d09-4999-a720-037c51640f19",
    "created_at": "2025-08-20T17:23:41.65783Z",
    "updated_at": "2025-08-20T18:07:26.491148Z",
    "email": "testmail@mail.com",
    "is_chirpy_red": false
}
```

And if the token is invalid, a 401 response will be given instead.

### POST /api/polka/webhooks
This endpoint is used by our payment provider 'Polka', we receive webhooks on it to upgrade users who paid for Chirpy Red subscriptions.

The request must have an `Authorization` header with the value `ApiKey {apiKey}`, where `{apiKey}` must match the environment variable `POLKA_KEY` to process the request.
It must also have a body of the form:
```json
{
  "event": "user.upgrade",
  "data": {
    "user_id": "{userID}"
  }
}
```
Where `{userID}` is the UUID of the user who paid for Chirpy Red and needs to be upgrade. And if the `event` is anything other than `user.upgrade`, the request will be ignored.

This endpoint gives a 204 response on a successful upgrade or ignored request (different event), and a 401 response if the `Authorization` header is invalid, or 404 if the given user UUID is not found in the database.


## Chirps
Chirps are Chirpy's version of posts (like tweets for Twitter), they can be created, viewed, or deleted.

### GET /api/chirps
This endpoint responds with a list of chirps, and takes 2 optional query parameters that affect the returned list:

 - `author_id={userID}`, will only show chirps which were posted by the user with UUID `{userID}`.
 - `sort={order}`, where order is either `asc` or `desc` (defaults to `asc` if omitted), orders returned chirps by creation time in (asc)ending or (desc)ending order/

Here is an example of a valid request to this endpoint:
```http request
GET http://localhost:8080/api/chirps?sort=desc&author_id=781fa276-8d09-4999-a720-037c51640f19
```
And the response body to that would look something like this:
```json
[
  {
    "id": "00fa39e2-ee5e-44db-b87d-2476fdd6ab8d",
    "created_at": "2025-08-20T19:00:20.940552Z",
    "updated_at": "2025-08-20T19:00:20.940552Z",
    "body": "test chirp 3",
    "user_id": "781fa276-8d09-4999-a720-037c51640f19"
  },
  {
    "id": "adbb24ca-e99b-427b-a4db-902a6c3381ca",
    "created_at": "2025-08-20T19:00:15.310602Z",
    "updated_at": "2025-08-20T19:00:15.310602Z",
    "body": "test chirp 2",
    "user_id": "781fa276-8d09-4999-a720-037c51640f19"
  },
  {
    "id": "2cb31ccb-9fae-41e0-b87f-dfc5a6bcede3",
    "created_at": "2025-08-20T19:00:08.049487Z",
    "updated_at": "2025-08-20T19:00:08.049487Z",
    "body": "test chirp 1",
    "user_id": "781fa276-8d09-4999-a720-037c51640f19"
  }
]
```

### GET /api/chirps/{chirpID}
This endpoint is used to retrieve a single chirp by its UUID. Responds with 404 if that chirp doesn't exist or if it was deleted.

A successful request's response body would look like this example:
```json
{
  "id": "2cb31ccb-9fae-41e0-b87f-dfc5a6bcede3",
  "created_at": "2025-08-20T19:00:08.049487Z",
  "updated_at": "2025-08-20T19:00:08.049487Z",
  "body": "test chirp 1",
  "user_id": "781fa276-8d09-4999-a720-037c51640f19"
}
```

### POST /api/chirps
This endpoint is used to create a new chirp, the given body must be no longer than 140 characters.

It requires an `Authorization` header with the value `Bearer {accessToken}`, where `{accessToken}` is a valid user's access token.
If the header is missing, or the provided access token is invalid, a 401 response is given.
Otherwise, if the given chirp body is longer than 140 characters, a 400 response is given with the following body:
```json
{
    "error": "Chirp is too long"
}
```

And finally, if the given access token is valid, and the chirp is no longer than 140 characters, a 201 response code is given with the created chirp as the body, like this example:
```json
{
  "id": "00fa39e2-ee5e-44db-b87d-2476fdd6ab8d",
  "created_at": "2025-08-20T19:00:20.940552Z",
  "updated_at": "2025-08-20T19:00:20.940552Z",
  "body": "test chirp 3",
  "user_id": "781fa276-8d09-4999-a720-037c51640f19"
}
```

### DELETE /api/chirps/{chirpID}
This endpoint is the opposite of the previous one, it deletes the chirp with UUID `{chirpID}`.

It requires an `Authorization` header with the value `Bearer {accessToken}`, where `{accessToken}` is the access token of the chirp's author.
If the header is missing, or the provided access token is invalid, a 401 response is given.
Otherwise, if the access token doesn't belong to the author of the chirp, a 403 response is given.

A 404 response is given if there is no chirp with UUID of `{chirpID}`, and if `{chirpID}` is not a valid UUID, a 400 response is given with the following body:
```json
{
    "error": "Invalid UUID"
}
```

Finally, if the chirp gets deleted successfully, a 204 response is given.

# Setup Guide:
## 1. Install Go:
As this is a Go project, you'll need to install Go on your machine to run it.
You can install go in either of these 2 ways:

- Using [Webi](https://webinstall.dev/golang/), this is the easiest way to do it in my opinion

- Or you could go to the [official Go installation page](https://go.dev/doc/install) and follow the instructions there for your platform

## 2. Create the database:
This project requires a PostgreSQL database to be connected. After installing Postgres and creating a database, you need to get the database connection string.
The Postgres connection string usually looks like this:
```
postgtres://username:password@localhost:5432/database
```
Where `username` and `password` are you database credentials that you set when creating the database, and `database` is the name of the database you created.

## 3. Clone the repository:
You need to clone this repo to your local machine in order to run it, you can do so by running the command:
```
git clone https://github.com/R0Xps/chirpy.git
```

## 4. Navigate to the project directory:
After cloning the project repository, you will end up with a new directory that contains everything you need to run it, navigate to that directory by running this command:
```shell
cd chirpy
```

## 4. Install Goose and the migrations:
Your database is probably empty now, and you need to set it up with the required tables for this project.
To do that, you can use a migration tool like [Goose](https://github.com/pressly/goose).

To install Goose, you can simply run this command:
```shell
go install github.com/pressly/goose/v3/cmd/goose@latest
```
Then run this command to run the migrations and get your database ready:
```shell
goose -dir ./sql/schema postgres <connection_string> up
```
Where `<connection_string>` is the connection string of your database that you got earlier.

## 5. Set environment variables:
Create a .env file in the root of your cloned Chirpy repo, and write the following in it:
```shell
DB_URL="connection_string"
PLATFORM="dev"
SECRET="secret"
POLKA_KEY="polkaKey"
```
 - `DB_URL`: the connection string of your database, used to store, retrieve, and update the data in it.
 - `PLATFORM`: only used by the `POST /admin/reset` and if it is set to anything other than `dev` (including if it's omitted), the endpoint will give a 403 response.
 - `SECRET`: used to hash the user passwords before storing them in the database.
 - `POLKA_KEY`: used to validate that webhooks we get on `POST /api/polka/webhooks` are from Polka. If the ApiKey we get with the webhooks matches the environment variable, the request is accepted, otherwise it is rejected.

## 6. Run the project:
Finally, to run the project, simply run this command from the root of your cloned repo:
```shell
go run .
```
Now the web server is running on your machine and can be accessed at http://localhost:8080.
