# Build the Pod Reaper app in stages

# Front end: Elm
FROM node:14.4.0 as frontend
RUN yarn global add create-elm-app@4.2.24
WORKDIR /app
COPY frontend/elm.json .
COPY frontend/public public/
COPY frontend/src src/
RUN ELM_APP_URL=/reaper/ elm-app build

# Back end: Golang
FROM golang:1.14.4-alpine as backend
WORKDIR /go/src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY cmd ./cmd
RUN go build -o reaper ./cmd/reaper

# Final image: Alpine
FROM alpine:3.12.0
RUN apk add --no-cache tzdata
WORKDIR /podreaper
COPY --from=frontend /app/build ./ui
COPY --from=backend /go/src/reaper ./reaper
ENV CORS_ENABLED false
ENV STATIC_FILES ./ui
CMD ["./reaper"]
