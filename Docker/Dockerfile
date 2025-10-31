FROM ubuntu
#FROM scratch
COPY nrlnanny /nrlnanny/
COPY nrlnanny.yaml /nrlnanny/
RUN mkdir -p /nrlnanny/conf
RUN mkdir -p /nrlnanny/data/license
RUN mkdir -p /nrlnanny/data/log
RUN apt update
RUN apt install libasound2-dev  tzdata -y
COPY start.sh /nrlnanny/
ENV TZ=Asia/Shanghai
ENTRYPOINT ["/nrlnanny/start.sh"]

