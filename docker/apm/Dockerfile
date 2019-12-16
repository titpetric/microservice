FROM docker.elastic.co/apm/apm-server:7.4.0

USER root
COPY apm-server.yml /usr/share/apm-server/apm-server.yml
RUN chown root:apm-server /usr/share/apm-server/apm-server.yml

USER apm-server
