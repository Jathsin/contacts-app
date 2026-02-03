FROM golang:latest AS build

WORKDIR /contacts-app
# Install dependencies
ADD go.mod .
ADD go.sum .
#executes command to download all dependencies described in go.mod file
RUN go mod download 


# add source code
ADD . .


# Build
# GOOS=linux GOARCH=amd64
# Because the VM machine is running on a different OS than the container we need to set the target OS and architecture
RUN CGO_ENABLED=0 go build -o contacts_app . 


# RUN server: executes binaries
FROM gcr.io/distroless/static-debian12
WORKDIR /contacts-app
COPY --from=build /contacts-app .

CMD ["./contacts_app"]