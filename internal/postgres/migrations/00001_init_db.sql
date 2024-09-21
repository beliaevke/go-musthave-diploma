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
    accrual real default 0,
    uploadedAt timestamp default NULL
);

CREATE TABLE OrdersOperations (
    userID bigint not null,
    orderNumber varchar(200) not null,
    pointsQuantity real default 0,
    processedAt timestamp default NULL
);

CREATE TABLE UsersBalance (
    userID bigint not null,
    pointsSum real default 0,
    pointsLoss real default 0
);

-- +goose Down
DROP TABLE Users;
DROP TABLE Orders;