# Tourist App (Flutter web, HTML renderer) served by nginx. Build: make app-build.
FROM ghcr.io/cirruslabs/flutter:3.24.0 AS build
ARG API_BASE=http://localhost:8000
ARG WEB_BASE=http://localhost:3000
ARG MOCKEXT_BASE=http://localhost:8090
ARG DEMO=true
WORKDIR /src
COPY apps/mobile/pubspec.yaml apps/mobile/pubspec.lock* ./
RUN flutter pub get
COPY apps/mobile/ ./
RUN flutter build web --release --web-renderer html --pwa-strategy none --no-web-resources-cdn \
      --dart-define=API_BASE=$API_BASE --dart-define=WEB_BASE=$WEB_BASE \
      --dart-define=MOCKEXT_BASE=$MOCKEXT_BASE --dart-define=DEMO=$DEMO

FROM nginx:1.27-alpine
COPY deploy/app-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/build/web /usr/share/nginx/html
