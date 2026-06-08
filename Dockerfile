FROM golang:1.24-alpine AS builder

RUN apk add --no-cache nodejs npm git ca-certificates

WORKDIR /app

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Cache npm deps
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN cd frontend && npm ci

# Copy rest of source
COPY . .

# Build frontend for go:embed
RUN cd frontend && npm run build

# Verify backend compilation
RUN go build -o /dev/null ./backend/...

FROM golang:1.24-alpine
WORKDIR /app
COPY --from=builder /app ./
CMD ["go", "test", "./backend/...", "-v", "-count=1"]
