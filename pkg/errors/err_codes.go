package errors

var ErrorCodes = map[ErrorType]int{
	// HTTP-Style Codes (4xx/5xx)
	ErrorTypeValidation:   400, // Bad Request
	ErrorTypeNotFound:     404, // Not Found
	ErrorTypeUnauthorized: 401, // Unauthorized
	ErrorTypeForbidden:    403, // Forbidden
	ErrorTypeConflict:     409, // Conflict
	ErrorTypePersistence:  500, // Internal Server Error
	ErrorTypeSystem:       500, // Internal Server Error

	// Custom/Application Errors (Negative Codes)

	// System/Connection Issues (-10xx)
	ErrTypeUnknown:      -1000, // UNKNOWN
	ErrTypeDisconnected: -1001, // DISCONNECTED
	// ErrTypeUnauthorized (was commented out in switch): -1002,
	ErrTypeTooManyRequests:         -1003, // TOO_MANY_REQUESTS
	ErrTypeUnexpectedResp:          -1006, // UNEXPECTED_RESP
	ErrTypeTimeout:                 -1007, // TIMEOUT
	ErrTypeServerBusy:              -1008, // SERVER_BUSY
	ErrTypeInvalidMessage:          -1013, // INVALID_MESSAGE
	ErrTypeUnknownOrderComposition: -1014, // UNKNOWN_ORDER_COMPOSITION
	ErrTypeTooManyOrders:           -1015, // TOO_MANY_ORDERS
	ErrTypeServiceShuttingDown:     -1016, // SERVICE_SHUTTING_DOWN
	ErrTypeUnsupportedOperation:    -1020, // UNSUPPORTED_OPERATION
	ErrTypeInvalidTimestamp:        -1021, // INVALID_TIMESTAMP
	ErrTypeInvalidSignature:        -1022, // INVALID_SIGNATURE
	ErrTypeCompIdInUse:             -1033, // COMP_ID_IN_USE
	ErrTypeTooManyConnections:      -1034, // TOO_MANY_CONNECTIONS
	ErrTypeLoggedOut:               -1035, // LOGGED_OUT

	// Request/Parameter Issues (-11xx)
	ErrTypeIllegalChars:                   -1100, // ILLEGAL_CHARS
	ErrTypeTooManyParameters:              -1101, // TOO_MANY_PARAMETERS
	ErrTypeMandatoryParamEmptyOrMalformed: -1102, // MANDATORY_PARAM_EMPTY_OR_MALFORMED
	ErrTypeUnknownParam:                   -1103, // UNKNOWN_PARAM
	ErrTypeUnreadParameters:               -1104, // UNREAD_PARAMETERS
	ErrTypeParamEmpty:                     -1105, // PARAM_EMPTY
	ErrTypeParamNotRequired:               -1106, // PARAM_NOT_REQUIRED
	ErrTypeParamOverflow:                  -1108, // PARAM_OVERFLOW
	ErrTypeBadPrecision:                   -1111, // BAD_PRECISION
	ErrTypeNoDepth:                        -1112, // NO_DEPTH
	ErrTypeTifNotRequired:                 -1114, // TIF_NOT_REQUIRED
	ErrTypeInvalidTif:                     -1115, // INVALID_TIF
	ErrTypeInvalidOrderType:               -1116, // INVALID_ORDER_TYPE
	ErrTypeInvalidSide:                    -1117, // INVALID_SIDE
	ErrTypeEmptyNewClOrdId:                -1118, // EMPTY_NEW_CL_ORD_ID
	ErrTypeEmptyOrgClOrdId:                -1119, // EMPTY_ORG_CL_ORD_ID
	ErrTypeBadInterval:                    -1120, // BAD_INTERVAL
	ErrTypeBadSymbol:                      -1121, // BAD_SYMBOL
	ErrTypeInvalidSymbolStatus:            -1122, // INVALID_SYMBOLSTATUS
	ErrTypeInvalidListenKey:               -1125, // INVALID_LISTEN_KEY
	ErrTypeMoreThanXXHours:                -1127, // MORE_THAN_XX_HOURS
	ErrTypeOptionalParamsBadCombo:         -1128, // OPTIONAL_PARAMS_BAD_COMBO
	ErrTypeInvalidParameter:               -1130, // INVALID_PARAMETER
	ErrTypeBadStrategyType:                -1134, // BAD_STRATEGY_TYPE
	ErrTypeInvalidJson:                    -1135, // INVALID_JSON
	ErrTypeInvalidTickerType:              -1139, // INVALID_TICKER_TYPE
	ErrTypeInvalidCancelRestrictions:      -1145, // INVALID_CANCEL_RESTRICTIONS
	ErrTypeDuplicateSymbols:               -1151, // DUPLICATE_SYMBOLS
	ErrTypeInvalidSbeHeader:               -1152, // INVALID_SBE_HEADER
	ErrTypeUnsupportedSchemaId:            -1153, // UNSUPPORTED_SCHEMA_ID
	ErrTypeSbeDisabled:                    -1155, // SBE_DISABLED
	ErrTypeOcoOrderTypeRejected:           -1158, // OCO_ORDER_TYPE_REJECTED
	ErrTypeOcoIcebergqtyTimeinforce:       -1160, // OCO_ICEBERGQTY_TIMEINFORCE
	ErrTypeDeprecatedSchema:               -1161, // DEPRECATED_SCHEMA
	ErrTypeBuyOcoLimitMustBeBelow:         -1165, // BUY_OCO_LIMIT_MUST_BE_BELOW
	ErrTypeSellOcoLimitMustBeAbove:        -1166, // SELL_OCO_LIMIT_MUST_BE_ABOVE
	ErrTypeBothOcoOrdersCannotBeLimit:     -1168, // BOTH_OCO_ORDERS_CANNOT_BE_LIMIT
	ErrTypeInvalidTagNumber:               -1169, // INVALID_TAG_NUMBER
	ErrTypeTagNotDefinedInMessage:         -1170, // TAG_NOT_DEFINED_IN_MESSAGE
	ErrTypeTagAppearsMoreThanOnce:         -1171, // TAG_APPEARS_MORE_THAN_ONCE
	ErrTypeTagOutOfOrder:                  -1172, // TAG_OUT_OF_ORDER
	ErrTypeGroupFieldsOutOfOrder:          -1173, // GROUP_FIELDS_OUT_OF_ORDER
	ErrTypeInvalidComponent:               -1174, // INVALID_COMPONENT
	ErrTypeResetSeqNumSupport:             -1175, // RESET_SEQ_NUM_SUPPORT
	ErrTypeAlreadyLoggedIn:                -1176, // ALREADY_LOGGED_IN
	ErrTypeGarbledMessage:                 -1177, // GARBLED_MESSAGE
	ErrTypeBadSenderCompId:                -1178, // BAD_SENDER_COMPID
	ErrTypeBadSeqNum:                      -1179, // BAD_SEQ_NUM
	ErrTypeExpectedLogon:                  -1180, // EXPECTED_LOGON
	ErrTypeTooManyMessages:                -1181, // TOO_MANY_MESSAGES
	ErrTypeParamsBadCombo:                 -1182, // PARAMS_BAD_COMBO
	ErrTypeNotAllowedInDropCopySessions:   -1183, // NOT_ALLOWED_IN_DROP_COPY_SESSIONS
	ErrTypeDropCopySessionNotAllowed:      -1184, // DROP_COPY_SESSION_NOT_ALLOWED
	ErrTypeDropCopySessionRequired:        -1185, // DROP_COPY_SESSION_REQUIRED
	ErrTypeNotAllowedInOrderEntrySessions: -1186, // NOT_ALLOWED_IN_ORDER_ENTRY_SESSIONS
	ErrTypeNotAllowedInMarketDataSessions: -1187, // NOT_ALLOWED_IN_MARKET_DATA_SESSIONS
	ErrTypeIncorrectNumInGroupCount:       -1188, // INCORRECT_NUM_IN_GROUP_COUNT
	ErrTypeDuplicateEntriesInAGroup:       -1189, // DUPLICATE_ENTRIES_IN_A_GROUP
	ErrTypeInvalidRequestId:               -1190, // INVALID_REQUEST_ID
	ErrTypeTooManySubscriptions:           -1191, // TOO_MANY_SUBSCRIPTIONS
	ErrTypeInvalidTimeUnit:                -1194, // INVALID_TIME_UNIT
	ErrTypeBuyOcoStopLossMustBeAbove:      -1196, // BUY_OCO_STOP_LOSS_MUST_BE_ABOVE
	ErrTypeSellOcoStopLossMustBeBelow:     -1197, // SELL_OCO_STOP_LOSS_MUST_BE_BELOW
	ErrTypeBuyOcoTakeProfitMustBeBelow:    -1198, // BUY_OCO_TAKE_PROFIT_MUST_BE_BELOW
	ErrTypeSellOcoTakeProfitMustBeAbove:   -1199, // SELL_OCO_TAKE_PROFIT_MUST_BE_ABOVE
	ErrTypeInvalidPegPriceType:            -1210, // INVALID_PEG_PRICE_TYPE
	ErrTypeInvalidPegOffsetType:           -1211, // INVALID_PEG_OFFSET_TYPE
	ErrTypeSymbolDoesNotMatchStatus:       -1220, // SYMBOL_DOES_NOT_MATCH_STATUS

	// Trading/Order Errors (-20xx)
	ErrTypeNewOrderRejected:                  -2010, // NEW_ORDER_REJECTED
	ErrTypeCancelRejected:                    -2011, // CANCEL_REJECTED
	ErrTypeNoSuchOrder:                       -2013, // NO_SUCH_ORDER
	ErrTypeBadApiKeyFmt:                      -2014, // BAD_API_KEY_FMT
	ErrTypeRejectedMbxKey:                    -2015, // REJECTED_MBX_KEY
	ErrTypeNoTradingWindow:                   -2016, // NO_TRADING_WINDOW
	ErrTypeOrderCancelReplacePartiallyFailed: -2021, // ORDER_CANCEL_REPLACE_PARTIALLY_FAILED
	ErrTypeOrderCancelReplaceFailed:          -2022, // ORDER_CANCEL_REPLACE_FAILED
	ErrTypeOrderArchived:                     -2026, // ORDER_ARCHIVED
	ErrTypeSubscriptionActive:                -2035, // SUBSCRIPTION_ACTIVE
	ErrTypeSubscriptionInactive:              -2036, // SUBSCRIPTION_INACTIVE
	ErrTypeClientOrderIdInvalid:              -2039, // CLIENT_ORDER_ID_INVALID
	ErrTypeMaximumSubscriptionIds:            -2042, // MAXIMUM_SUBSCRIPTION_IDS
}
