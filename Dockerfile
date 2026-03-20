# Start by building the application.
FROM golang:1.21 AS build

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/app ./container/main.go

# Now copy it into our base image.
FROM asia-east1-docker.pkg.dev/muulin-universal/ml-base/app:main
COPY --from=build /go/bin/app /