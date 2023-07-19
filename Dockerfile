FROM ubuntu:22.04
RUN  mkdir -p /app/config /app/tls /tls
WORKDIR /app
COPY main init.sh /app/
VOLUME /app/config /app/tls /tls
EXPOSE 80 443
CMD ["./init.sh"]
