FROM alpine:latest

ARG service_name
ENV service_name=$service_name

WORKDIR /app

ENV TZ Europe/Ljubljana
RUN apk --no-cache add ca-certificates tzdata && ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

COPY /build/${service_name}-linux-amd64 /app/service

RUN adduser -S www-data -u 1000

# delete all the bundled binaries on standard PATH locations
RUN rm -rf /bin /sbin /usr/bin /usr/sbin /usr/local/bin /usr/local/sbin

USER www-data

EXPOSE 3000

ENTRYPOINT ["/app/service"]
