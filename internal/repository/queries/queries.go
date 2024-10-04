package queries

////////////////////////////////////////
// balancerepo

const GetBalanceQueryRow = `
		SELECT pointsSum, pointsLoss
		FROM
			public.usersbalance
		WHERE
			usersbalance.userID=$1
	`

const BalanceWithdrawInsert = `
		INSERT INTO public.ordersoperations
		(userID, orderNumber, pointsQuantity, processedAt)
		VALUES
		($1, $2, $3, $4)
	`

const BalanceWithdrawUpdate = `
		UPDATE public.usersbalance
		SET pointssum=$1, pointsloss=$2
		WHERE userID=$3;
	`

const GetWithdrawalsQuery = `
		SELECT orderNumber, -pointsQuantity as pointsQuantity, processedAt
		FROM
			public.ordersoperations
		WHERE
			ordersoperations.userID=$1 AND ordersoperations.pointsQuantity < 0
		ORDER BY
			ordersoperations.processedAt DESC
	`

////////////////////////////////////////
// ordersrepo

const GetOrderQueryRow = `
		SELECT orders.userID
		FROM
			public.orders
		WHERE
			orders.ordernumber=$1
	`

const AddOrderInsert = `
		INSERT INTO public.orders
		(userID, ordernumber, orderstatus, uploadedat)
		VALUES
		($1, $2, $3, $4);
	`

const GetOrdersQueryRow = `
		SELECT ordernumber, orderstatus, accrual, uploadedat
		FROM
			public.orders
		WHERE
			orders.userID=$1
		ORDER BY
			orders.uploadedat DESC
	`

const GetAwaitOrdersQueryRow = `
		SELECT ordernumber, orderstatus, accrual, uploadedat
		FROM
			public.orders
		WHERE
			orders.orderstatus != 'INVALID' AND orders.orderstatus != 'PROCESSED'
		ORDER BY
			orders.uploadedat
	`

const UpdateOrderInsert = `
		INSERT INTO public.ordersoperations
		(userID, orderNumber, pointsQuantity, processedAt)
		VALUES
		($1, $2, $3, $4)
		`

const UpdateOrderQuery = `
		UPDATE public.orders
		SET orderStatus=$1, accrual=$2, uploadedAt=$3
		WHERE userID=$4 AND orderNumber=$5;
	`

const UpdateBalanceQuery = `
		UPDATE public.usersbalance
		SET pointssum=$1
		WHERE userID=$2;
	`

////////////////////////////////////////
// usersrepo

const SelectUser = `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1
	`

const SelectUserWithPass = `
		SELECT users.userID
		FROM
			public.users
		WHERE
		users.userLogin=$1 AND users.userPassword = $2
	`

const CreateUserInsert = `
		INSERT INTO public.users
		(userLogin, userPassword)
		VALUES
		($1, $2);
	`

const CreateUserBalanceInsert = `
		INSERT INTO public.usersbalance
		(userID, pointsSum, pointsLoss)
		VALUES
		($1, $2, $3);
	`
