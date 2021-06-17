# Based on https://flant.slack.com/archives/C4U78TPC1/p1622732293021600
ARG BASE_UBUNTU
FROM okmeter/okagent:latest@sha256:52edf71d312b337a84c5a095105e1eb985d0341f04b5c6a306f60a92c88f4148 as artifact

FROM $BASE_UBUNTU
RUN apt-get update && apt-get install -y ca-certificates curl iputils-ping gnupg gnupg2 gnupg1 && rm -rf /var/lib/apt/lists/*
RUN apt-get update && apt-get install -y openjdk-8-jre-headless && rm -rf /var/lib/apt/lists/*
RUN apt-get update && apt-get install -y smartmontools && rm -rf /var/lib/apt/lists/*
RUN echo "deb http://hwraid.le-vert.net/ubuntu bionic main" > /etc/apt/sources.list.d/hwraid.list && \
	curl -sSL -o - https://hwraid.le-vert.net/debian/hwraid.le-vert.net.gpg.key | apt-key add - && \
	apt-get update && apt-get install megacli arcconf && rm -rf /var/lib/apt/lists/*
COPY --from=artifact /usr/local/bin/oksupervisor /usr/local/bin/oksupervisor
VOLUME /usr/local/okagent
ENV OKMETER_AGENT_INCONTAINER true
#statsd
EXPOSE 8125/udp
#pinba
EXPOSE 30002/udp
CMD ["oksupervisor"]
