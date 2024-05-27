-- +goose Up
CREATE TABLE Users (
    userID bigint primary key generated always as identity,
    userLogin varchar(200) not null unique,
    userPassword varchar(200) not null
);

CREATE TABLE Orders (
    userID bigint not null,
    orderNumber varchar(200) not null unique,
    orderStatus varchar(20),
    accrual int,
    uploadedAt timestamp default NULL
);

-- +goose Down
DROP TABLE Users;
DROP TABLE Orders;