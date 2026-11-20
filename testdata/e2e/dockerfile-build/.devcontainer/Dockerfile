# syntax=docker/dockerfile:1
ARG BASE=debian:bookworm-slim
FROM ${BASE} AS dev
ARG GREETING=unset
COPY tools/greet.sh /usr/local/bin/greet
RUN chmod 0755 /usr/local/bin/greet && echo "$GREETING" > /etc/greeting

# build.target selects dev; building on this stage instead must fail.
FROM dev AS ci
RUN echo "dc2kit built past build.target" && false
