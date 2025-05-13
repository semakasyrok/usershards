package models

type TransactionType string

const TransactionTypeDecrease TransactionType = "decrease"
const TransactionTypeIncrease TransactionType = "increase"
const TransactionTypeCompensate TransactionType = "compensate"
