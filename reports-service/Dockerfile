FROM golang:1.25.5 AS builder

ENV BIN_FILE="/opt/reports-service/reports-service"
ENV CODE_DIR="/go/src/"

WORKDIR ${CODE_DIR}

# Copy go mod and sum files
COPY go.mod go.sum ${CODE_DIR}

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . ${CODE_DIR}

# Собираем статический бинарник Go (без зависимостей на Си API),
# иначе он не будет работать в alpine образе.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -o ${BIN_FILE} cmd/reports-service/*

# Use a smaller image for the final application
FROM alpine:latest  

RUN apk --no-cache add ca-certificates

ENV BIN_FILE="/opt/reports-service/reports-service"

# Copy the binary from builder
COPY --from=builder ${BIN_FILE} ${BIN_FILE}

ENV CONFIG_FILE="/etc/reports-service/config.yaml"
COPY ./configs/config.yaml ${CONFIG_FILE}

# Expose the application port
EXPOSE 8001

# Command to run the executable
CMD ${BIN_FILE} -config ${CONFIG_FILE}