# Build the Pod Reaper app in stages

# Front end: Elm
FROM node:23.1.0 as frontend
WORKDIR /app
RUN npm i --save-dev parcel@2.12.0 @parcel/transformer-elm@2.12.0
COPY frontend/elm.json .
COPY frontend/src src/
RUN echo ELM_APP_URL="/reaper/" > .env
RUN npx parcel build src/index.html

# Back end: Golang
FROM golang:1.23.3-alpine as backend
WORKDIR /go/src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY cmd ./cmd
RUN go build -o reaper ./cmd/reaper

# Final image: Alpine
FROM alpine:3.20.3
RUN apk add --no-cache tzdata
WORKDIR /podreaper
COPY --from=frontend /app/dist ./ui
COPY --from=backend /go/src/reaper ./reaper
ENV CORS_ENABLED false
ENV STATIC_FILES ./ui
CMD ["./reaper"]
