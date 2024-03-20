FROM centos:7.9.2009
WORKDIR /root/server
COPY . ./
RUN /bin/cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime &&\
    echo 'Asia/Shanghai' >/etc/timezone &&\
    ulimit -c unlimited &&\
    mkdir -p /tmp/coredump
CMD ["appService","1"]
