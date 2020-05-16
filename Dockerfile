# Build the Pod Reaper app in stages

FROM golang:1.14.2-alpine
# RUN mkdir -p app

WORKDIR /go/src
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY cmd ./cmd
RUN go build -o reaper ./cmd/reaper
# CMD ls -alh && cat go.mod
CMD ["/go/src/reaper"]


# Front end: Elm
# FROM node:14.2.0 as frontend
# RUN yarn global add create-elm-app@4.2.16
# WORKDIR /app
# COPY frontend/elm.json .
# COPY frontend/public public/
# COPY frontend/src src/
# RUN ELM_APP_URL=/reaper/ elm-app build

# Back end: Micronaut/Kotlin/Gradle
# FROM gradle:6.4.0 as backend
# WORKDIR /podreaper
# COPY build.gradle .
# COPY settings.gradle .
# COPY gradle.properties .
# change gradle home folder so cache will be preserved
# RUN gradle -g . shadowJar clean
# COPY src src
# RUN gradle -g . shadowJar
# e.g. /podreaper/build/libs/podreaper-1.0-SNAPSHOT-all.jar

# Final image: OpenJDK
# FROM adoptopenjdk:13-jre-hotspot
# WORKDIR /podreaper
# COPY --from=frontend /app/build ./ui
# COPY --from=backend /podreaper/build/libs/podreaper-*-all.jar podreaper.jar
# ENV CORS_ENABLED false
# CMD ["java", "-jar", "podreaper.jar"]
