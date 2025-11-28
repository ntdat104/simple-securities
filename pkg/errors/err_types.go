package errors

type ErrorType string

const (
	// Application Errors
	ErrorTypeValidation   ErrorType = "VALIDATION"   // ErrorTypeValidation represents validation errors
	ErrorTypeNotFound     ErrorType = "NOT_FOUND"    // ErrorTypeNotFound represents resource not found errors
	ErrorTypePersistence  ErrorType = "PERSISTENCE"  // ErrorTypePersistence represents persistence layer errors
	ErrorTypeSystem       ErrorType = "SYSTEM"       // ErrorTypeSystem represents internal system errors
	ErrorTypeBusiness     ErrorType = "BUSINESS"     // ErrorTypeBusiness represents business logic errors
	ErrorTypeUnauthorized ErrorType = "UNAUTHORIZED" // ErrorTypeUnauthorized represents authentication/authorization errors
	ErrorTypeForbidden    ErrorType = "FORBIDDEN"    // ErrorTypeForbidden represents permission errors
	ErrorTypeConflict     ErrorType = "CONFLICT"     // ErrorTypeConflict represents resource conflict errors

	// General/Internal Errors
	ErrTypeUnknown                 ErrorType = "UNKNOWN"                   // -1000 UNKNOWN
	ErrTypeDisconnected            ErrorType = "DISCONNECTED"              // -1001 DISCONNECTED
	ErrTypeUnauthorized            ErrorType = "UNAUTHORIZED"              // -1002 UNAUTHORIZED
	ErrTypeTooManyRequests         ErrorType = "TOO_MANY_REQUESTS"         // -1003 TOO_MANY_REQUESTS
	ErrTypeUnexpectedResp          ErrorType = "UNEXPECTED_RESP"           // -1006 UNEXPECTED_RESP
	ErrTypeTimeout                 ErrorType = "TIMEOUT"                   // -1007 TIMEOUT
	ErrTypeServerBusy              ErrorType = "SERVER_BUSY"               // -1008 SERVER_BUSY
	ErrTypeInvalidMessage          ErrorType = "INVALID_MESSAGE"           // -1013 INVALID_MESSAGE
	ErrTypeUnknownOrderComposition ErrorType = "UNKNOWN_ORDER_COMPOSITION" // -1014 UNKNOWN_ORDER_COMPOSITION
	ErrTypeTooManyOrders           ErrorType = "TOO_MANY_ORDERS"           // -1015 TOO_MANY_ORDERS
	ErrTypeServiceShuttingDown     ErrorType = "SERVICE_SHUTTING_DOWN"     // -1016 SERVICE_SHUTTING_DOWN
	ErrTypeUnsupportedOperation    ErrorType = "UNSUPPORTED_OPERATION"     // -1020 UNSUPPORTED_OPERATION
	ErrTypeInvalidTimestamp        ErrorType = "INVALID_TIMESTAMP"         // -1021 INVALID_TIMESTAMP
	ErrTypeInvalidSignature        ErrorType = "INVALID_SIGNATURE"         // -1022 INVALID_SIGNATURE
	ErrTypeCompIdInUse             ErrorType = "COMP_ID_IN_USE"            // -1033 COMP_ID_IN_USE
	ErrTypeTooManyConnections      ErrorType = "TOO_MANY_CONNECTIONS"      // -1034 TOO_MANY_CONNECTIONS
	ErrTypeLoggedOut               ErrorType = "LOGGED_OUT"                // -1035 LOGGED_OUT

	// Request/Parameter Issues (11xx)
	ErrTypeIllegalChars                   ErrorType = "ILLEGAL_CHARS"                       // -1100 ILLEGAL_CHARS
	ErrTypeTooManyParameters              ErrorType = "TOO_MANY_PARAMETERS"                 // -1101 TOO_MANY_PARAMETERS
	ErrTypeMandatoryParamEmptyOrMalformed ErrorType = "MANDATORY_PARAM_EMPTY_OR_MALFORMED"  // -1102 MANDATORY_PARAM_EMPTY_OR_MALFORMED
	ErrTypeUnknownParam                   ErrorType = "UNKNOWN_PARAM"                       // -1103 UNKNOWN_PARAM
	ErrTypeUnreadParameters               ErrorType = "UNREAD_PARAMETERS"                   // -1104 UNREAD_PARAMETERS
	ErrTypeParamEmpty                     ErrorType = "PARAM_EMPTY"                         // -1105 PARAM_EMPTY
	ErrTypeParamNotRequired               ErrorType = "PARAM_NOT_REQUIRED"                  // -1106 PARAM_NOT_REQUIRED
	ErrTypeParamOverflow                  ErrorType = "PARAM_OVERFLOW"                      // -1108 PARAM_OVERFLOW
	ErrTypeBadPrecision                   ErrorType = "BAD_PRECISION"                       // -1111 BAD_PRECISION
	ErrTypeNoDepth                        ErrorType = "NO_DEPTH"                            // -1112 NO_DEPTH
	ErrTypeTifNotRequired                 ErrorType = "TIF_NOT_REQUIRED"                    // -1114 TIF_NOT_REQUIRED
	ErrTypeInvalidTif                     ErrorType = "INVALID_TIF"                         // -1115 INVALID_TIF
	ErrTypeInvalidOrderType               ErrorType = "INVALID_ORDER_TYPE"                  // -1116 INVALID_ORDER_TYPE
	ErrTypeInvalidSide                    ErrorType = "INVALID_SIDE"                        // -1117 INVALID_SIDE
	ErrTypeEmptyNewClOrdId                ErrorType = "EMPTY_NEW_CL_ORD_ID"                 // -1118 EMPTY_NEW_CL_ORD_ID
	ErrTypeEmptyOrgClOrdId                ErrorType = "EMPTY_ORG_CL_ORD_ID"                 // -1119 EMPTY_ORG_CL_ORD_ID
	ErrTypeBadInterval                    ErrorType = "BAD_INTERVAL"                        // -1120 BAD_INTERVAL
	ErrTypeBadSymbol                      ErrorType = "BAD_SYMBOL"                          // -1121 BAD_SYMBOL
	ErrTypeInvalidSymbolStatus            ErrorType = "INVALID_SYMBOLSTATUS"                // -1122 INVALID_SYMBOLSTATUS
	ErrTypeInvalidListenKey               ErrorType = "INVALID_LISTEN_KEY"                  // -1125 INVALID_LISTEN_KEY
	ErrTypeMoreThanXXHours                ErrorType = "MORE_THAN_XX_HOURS"                  // -1127 MORE_THAN_XX_HOURS
	ErrTypeOptionalParamsBadCombo         ErrorType = "OPTIONAL_PARAMS_BAD_COMBO"           // -1128 OPTIONAL_PARAMS_BAD_COMBO
	ErrTypeInvalidParameter               ErrorType = "INVALID_PARAMETER"                   // -1130 INVALID_PARAMETER
	ErrTypeBadStrategyType                ErrorType = "BAD_STRATEGY_TYPE"                   // -1134 BAD_STRATEGY_TYPE
	ErrTypeInvalidJson                    ErrorType = "INVALID_JSON"                        // -1135 INVALID_JSON
	ErrTypeInvalidTickerType              ErrorType = "INVALID_TICKER_TYPE"                 // -1139 INVALID_TICKER_TYPE
	ErrTypeInvalidCancelRestrictions      ErrorType = "INVALID_CANCEL_RESTRICTIONS"         // -1145 INVALID_CANCEL_RESTRICTIONS
	ErrTypeDuplicateSymbols               ErrorType = "DUPLICATE_SYMBOLS"                   // -1151 DUPLICATE_SYMBOLS
	ErrTypeInvalidSbeHeader               ErrorType = "INVALID_SBE_HEADER"                  // -1152 INVALID_SBE_HEADER
	ErrTypeUnsupportedSchemaId            ErrorType = "UNSUPPORTED_SCHEMA_ID"               // -1153 UNSUPPORTED_SCHEMA_ID
	ErrTypeSbeDisabled                    ErrorType = "SBE_DISABLED"                        // -1155 SBE_DISABLED
	ErrTypeOcoOrderTypeRejected           ErrorType = "OCO_ORDER_TYPE_REJECTED"             // -1158 OCO_ORDER_TYPE_REJECTED
	ErrTypeOcoIcebergqtyTimeinforce       ErrorType = "OCO_ICEBERGQTY_TIMEINFORCE"          // -1160 OCO_ICEBERGQTY_TIMEINFORCE
	ErrTypeDeprecatedSchema               ErrorType = "DEPRECATED_SCHEMA"                   // -1161 DEPRECATED_SCHEMA
	ErrTypeBuyOcoLimitMustBeBelow         ErrorType = "BUY_OCO_LIMIT_MUST_BE_BELOW"         // -1165 BUY_OCO_LIMIT_MUST_BE_BELOW
	ErrTypeSellOcoLimitMustBeAbove        ErrorType = "SELL_OCO_LIMIT_MUST_BE_ABOVE"        // -1166 SELL_OCO_LIMIT_MUST_BE_ABOVE
	ErrTypeBothOcoOrdersCannotBeLimit     ErrorType = "BOTH_OCO_ORDERS_CANNOT_BE_LIMIT"     // -1168 BOTH_OCO_ORDERS_CANNOT_BE_LIMIT
	ErrTypeInvalidTagNumber               ErrorType = "INVALID_TAG_NUMBER"                  // -1169 INVALID_TAG_NUMBER
	ErrTypeTagNotDefinedInMessage         ErrorType = "TAG_NOT_DEFINED_IN_MESSAGE"          // -1170 TAG_NOT_DEFINED_IN_MESSAGE
	ErrTypeTagAppearsMoreThanOnce         ErrorType = "TAG_APPEARS_MORE_THAN_ONCE"          // -1171 TAG_APPEARS_MORE_THAN_ONCE
	ErrTypeTagOutOfOrder                  ErrorType = "TAG_OUT_OF_ORDER"                    // -1172 TAG_OUT_OF_ORDER
	ErrTypeGroupFieldsOutOfOrder          ErrorType = "GROUP_FIELDS_OUT_OF_ORDER"           // -1173 GROUP_FIELDS_OUT_OF_ORDER
	ErrTypeInvalidComponent               ErrorType = "INVALID_COMPONENT"                   // -1174 INVALID_COMPONENT
	ErrTypeResetSeqNumSupport             ErrorType = "RESET_SEQ_NUM_SUPPORT"               // -1175 RESET_SEQ_NUM_SUPPORT
	ErrTypeAlreadyLoggedIn                ErrorType = "ALREADY_LOGGED_IN"                   // -1176 ALREADY_LOGGED_IN
	ErrTypeGarbledMessage                 ErrorType = "GARBLED_MESSAGE"                     // -1177 GARBLED_MESSAGE
	ErrTypeBadSenderCompId                ErrorType = "BAD_SENDER_COMPID"                   // -1178 BAD_SENDER_COMPID
	ErrTypeBadSeqNum                      ErrorType = "BAD_SEQ_NUM"                         // -1179 BAD_SEQ_NUM
	ErrTypeExpectedLogon                  ErrorType = "EXPECTED_LOGON"                      // -1180 EXPECTED_LOGON
	ErrTypeTooManyMessages                ErrorType = "TOO_MANY_MESSAGES"                   // -1181 TOO_MANY_MESSAGES
	ErrTypeParamsBadCombo                 ErrorType = "PARAMS_BAD_COMBO"                    // -1182 PARAMS_BAD_COMBO
	ErrTypeNotAllowedInDropCopySessions   ErrorType = "NOT_ALLOWED_IN_DROP_COPY_SESSIONS"   // -1183 NOT_ALLOWED_IN_DROP_COPY_SESSIONS
	ErrTypeDropCopySessionNotAllowed      ErrorType = "DROP_COPY_SESSION_NOT_ALLOWED"       // -1184 DROP_COPY_SESSION_NOT_ALLOWED
	ErrTypeDropCopySessionRequired        ErrorType = "DROP_COPY_SESSION_REQUIRED"          // -1185 DROP_COPY_SESSION_REQUIRED
	ErrTypeNotAllowedInOrderEntrySessions ErrorType = "NOT_ALLOWED_IN_ORDER_ENTRY_SESSIONS" // -1186 NOT_ALLOWED_IN_ORDER_ENTRY_SESSIONS
	ErrTypeNotAllowedInMarketDataSessions ErrorType = "NOT_ALLOWED_IN_MARKET_DATA_SESSIONS" // -1187 NOT_ALLOWED_IN_MARKET_DATA_SESSIONS
	ErrTypeIncorrectNumInGroupCount       ErrorType = "INCORRECT_NUM_IN_GROUP_COUNT"        // -1188 INCORRECT_NUM_IN_GROUP_COUNT
	ErrTypeDuplicateEntriesInAGroup       ErrorType = "DUPLICATE_ENTRIES_IN_A_GROUP"        // -1189 DUPLICATE_ENTRIES_IN_A_GROUP
	ErrTypeInvalidRequestId               ErrorType = "INVALID_REQUEST_ID"                  // -1190 INVALID_REQUEST_ID
	ErrTypeTooManySubscriptions           ErrorType = "TOO_MANY_SUBSCRIPTIONS"              // -1191 TOO_MANY_SUBSCRIPTIONS
	ErrTypeInvalidTimeUnit                ErrorType = "INVALID_TIME_UNIT"                   // -1194 INVALID_TIME_UNIT
	ErrTypeBuyOcoStopLossMustBeAbove      ErrorType = "BUY_OCO_STOP_LOSS_MUST_BE_ABOVE"     // -1196 BUY_OCO_STOP_LOSS_MUST_BE_ABOVE
	ErrTypeSellOcoStopLossMustBeBelow     ErrorType = "SELL_OCO_STOP_LOSS_MUST_BE_BELOW"    // -1197 SELL_OCO_STOP_LOSS_MUST_BE_BELOW
	ErrTypeBuyOcoTakeProfitMustBeBelow    ErrorType = "BUY_OCO_TAKE_PROFIT_MUST_BE_BELOW"   // -1198 BUY_OCO_TAKE_PROFIT_MUST_BE_BELOW
	ErrTypeSellOcoTakeProfitMustBeAbove   ErrorType = "SELL_OCO_TAKE_PROFIT_MUST_BE_ABOVE"  // -1199 SELL_OCO_TAKE_PROFIT_MUST_BE_ABOVE
	ErrTypeInvalidPegPriceType            ErrorType = "INVALID_PEG_PRICE_TYPE"              // -1210 INVALID_PEG_PRICE_TYPE
	ErrTypeInvalidPegOffsetType           ErrorType = "INVALID_PEG_OFFSET_TYPE"             // -1211 INVALID_PEG_OFFSET_TYPE
	ErrTypeSymbolDoesNotMatchStatus       ErrorType = "SYMBOL_DOES_NOT_MATCH_STATUS"        // -1220 SYMBOL_DOES_NOT_MATCH_STATUS

	// Trading/Order Errors (2xxx)
	ErrTypeNewOrderRejected                  ErrorType = "NEW_ORDER_REJECTED"                    // -2010 NEW_ORDER_REJECTED
	ErrTypeCancelRejected                    ErrorType = "CANCEL_REJECTED"                       // -2011 CANCEL_REJECTED
	ErrTypeNoSuchOrder                       ErrorType = "NO_SUCH_ORDER"                         // -2013 NO_SUCH_ORDER
	ErrTypeBadApiKeyFmt                      ErrorType = "BAD_API_KEY_FMT"                       // -2014 BAD_API_KEY_FMT
	ErrTypeRejectedMbxKey                    ErrorType = "REJECTED_MBX_KEY"                      // -2015 REJECTED_MBX_KEY
	ErrTypeNoTradingWindow                   ErrorType = "NO_TRADING_WINDOW"                     // -2016 NO_TRADING_WINDOW
	ErrTypeOrderCancelReplacePartiallyFailed ErrorType = "ORDER_CANCEL_REPLACE_PARTIALLY_FAILED" // -2021 Order cancel-replace partially failed
	ErrTypeOrderCancelReplaceFailed          ErrorType = "ORDER_CANCEL_REPLACE_FAILED"           // -2022 Order cancel-replace failed
	ErrTypeOrderArchived                     ErrorType = "ORDER_ARCHIVED"                        // -2026 ORDER_ARCHIVED
	ErrTypeSubscriptionActive                ErrorType = "SUBSCRIPTION_ACTIVE"                   // -2035 SUBSCRIPTION_ACTIVE
	ErrTypeSubscriptionInactive              ErrorType = "SUBSCRIPTION_INACTIVE"                 // -2036 SUBSCRIPTION_INACTIVE
	ErrTypeClientOrderIdInvalid              ErrorType = "CLIENT_ORDER_ID_INVALID"               // -2039 CLIENT_ORDER_ID_INVALID
	ErrTypeMaximumSubscriptionIds            ErrorType = "MAXIMUM_SUBSCRIPTION_IDS"              // -2042 MAXIMUM_SUBSCRIPTION_IDS
)
