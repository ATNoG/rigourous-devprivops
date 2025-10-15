# Use image with golang and docker
FROM 192.168.56.1:5000/golang-fuseki:latest
# FROM golang-fuseki

COPY . /src/
WORKDIR /src/ 

# Build the application
RUN ls
RUN go mod tidy
RUN go build

# Move queries and configs elsewhere
RUN mkdir /etc/devprivops/
# RUN mv .devprivops/* /etc/devprivops/

# Cleanup
RUN mv devprivops /bin/devprivops
RUN rm -rf /src/
