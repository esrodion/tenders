FROM golang:alpine

WORKDIR /tenders
COPY . .

EXPOSE 8080

ENV MIGRATIONS_URL="file:///tenders/internal/repository/db/migrations/"

RUN ["go", "mod", "tidy"]
RUN ["go", "build", "-o", "tenders", "./cmd/..."]

CMD ["./tenders"]