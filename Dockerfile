FROM node:22-bookworm-slim AS builder

WORKDIR /build
COPY web/package.json .
RUN npm install --legacy-peer-deps
COPY ./web .
COPY ./VERSION .
RUN NODE_OPTIONS=--max-old-space-size=3072 DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$(cat VERSION) npm run build

FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder2
ENV GO111MODULE=on CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

ADD go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=builder /build/dist ./web/dist
RUN go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=$(cat VERSION)'" -o new-api

FROM debian:bookworm-slim@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      ca-certificates \
      tzdata \
      libasan8 \
      wget \
      libglib2.0-0 \
      libnss3 \
      libnspr4 \
      libdbus-1-3 \
      libatk1.0-0 \
      libatk-bridge2.0-0 \
      libcups2 \
      libdrm2 \
      libxkbcommon0 \
      libxcomposite1 \
      libxdamage1 \
      libxfixes3 \
      libxrandr2 \
      libgbm1 \
      libasound2 \
      libatspi2.0-0 \
      libx11-6 \
      libx11-xcb1 \
      libxcb1 \
      libxext6 \
      libxshmfence1 \
      libxss1 \
      libxtst6 \
      libgtk-3-0 \
      libpango-1.0-0 \
      libcairo2 \
      libxrender1 \
      fonts-liberation \
      libgl1-mesa-dri \
      libgl1-mesa-glx \
      mesa-vulkan-drivers \
      xvfb \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=builder2 /build/new-api /
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/docker-entrypoint.sh"]
