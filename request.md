curl -X POST http://localhost:8000/users/create \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john_doe",
    "email": "john@example.com",
    "full_name": "John Doe"
  }'

curl -X POST http://localhost:8000/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "abhishek"
  }'

curl -X POST "http://127.0.0.1:8000/test/scrape" \
  -H "accept: application/json" \
  -H "Authorization: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NTk0ODQ3MjIsInVzZXJfaWQiOjF9.MrbSHk9XZ6eQm0KGd6lKQxVFr36snDQl5xsUZ5Q0kLo" \
  -d '{
    "url": "AI Trends",
    "word_limit": 100
  }'

curl -X POST "http://127.0.0.1:8000/test/query/web" \
  -H "accept: application/json" \
  -H "Authorization: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NTk0ODQ3MjIsInVzZXJfaWQiOjF9.MrbSHk9XZ6eQm0KGd6lKQxVFr36snDQl5xsUZ5Q0kLo" \
  -d '{
    "queries": ["AI Trends"],
    "result_limit": 100
  }'



curl -X GET "http://127.0.0.1:8000/users/fetch/1" \
  -H "accept: application/json" \
  -H "Authorization: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NTk0ODQ3MjIsInVzZXJfaWQiOjF9.MrbSHk9XZ6eQm0KGd6lKQxVFr36snDQl5xsUZ5Q0kLo"


curl -X GET "http://127.0.0.1:8000/users/fetch" \
  -H "accept: application/json" \
  -H "Authorization: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3NTk0ODQ3MjIsInVzZXJfaWQiOjF9.MrbSHk9XZ6eQm0KGd6lKQxVFr36snDQl5xsUZ5Q0kLo"

