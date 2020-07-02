FROM registry.access.redhat.com/ubi8/ubi-minimal:latest as builder

RUN microdnf -y install go ca-certificates

WORKDIR /namespace-admission-controller

# load static dependencies to speed build
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/namespace-admission-controller

# Runtime image
FROM scratch AS base
COPY --from=builder /etc/pki /etc/ssl /etc/
COPY --from=builder /go/bin/namespace-admission-controller /bin/namespace-admission-controller
ENTRYPOINT ["/bin/namespace-admission-controller"]
