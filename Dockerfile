FROM scratch

WORKDIR /app

COPY ./search .
COPY ./config ./config
COPY ./localtime /etc/localtime

ENTRYPOINT ["./search"]
