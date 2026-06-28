FROM golang:1.26-bookworm AS builder

ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/codeseek ./cmd/codeseek

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/codeseek /app/codeseek
COPY config.example.yml /app/config.example.yml

EXPOSE 38440

USER nonroot:nonroot
ENTRYPOINT ["/app/codeseek"]
CMD ["-config", "/config/config.yml", "-addr", "0.0.0.0:38440"]
