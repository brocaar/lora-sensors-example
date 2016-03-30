FROM golang:1.6.0

ENV PROJECT_PATH=/go/src/github.com/brocaar/lora-sensors-example
ENV PATH=$PATH:$PROJECT_PATH/bin

# setup work directory
RUN mkdir -p $PROJECT_PATH
WORKDIR $PROJECT_PATH

# copy source code
COPY . $PROJECT_PATH

# install requirements
RUN go get .

# build
RUN go build

CMD ["lora-sensors-example"]
