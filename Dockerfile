FROM golang:stretch
COPY . /app
WORKDIR /app
RUN make dist