namespace go order

// QuoteRequest shows how a Thrift field qualifier maps onto Go after codegen.
//
// The field numbers (1:, 2:, ...) are the on-the-wire identity of each field.
// They are assigned ONCE and never reused: to drop a field you retire its
// number, you never hand it to a different field. That rule is what lets old
// and new peers talk to each other.
//
//   required T  -> generated as a value field (T). Always present on the wire.
//   optional T  -> generated as a POINTER (*T). nil means "the caller omitted
//                  this field", which is different from the zero value.
struct QuoteRequest {
    1: required string sku          // required -> Sku string
    2: required i64 quantity        // required -> Quantity int64

    // v2 additions. Adding OPTIONAL fields is backward compatible: an old
    // client that never sets them sends nothing, and the generated *string /
    // *i64 arrive as nil, so the handler can fall back to a default.
    3: optional string currency     // optional -> Currency *string (nil = caller omitted)
    4: optional i64 discount_bps    // optional -> DiscountBps *int64 (nil = no discount, vs 0 = explicit zero)
}

struct QuoteResponse {
    1: required string sku
    2: required i64 total_cents
    3: required string message
    4: optional string currency     // v2: echoes the resolved currency back
}

// PingRequest / PingResponse back the v2 method added to the service below.
struct PingRequest {}

struct PingResponse {
    1: required string status
}

service OrderService {
    QuoteResponse Quote(1: QuoteRequest req)

    // v2 addition. Adding a NEW METHOD is backward compatible: clients that
    // only know Quote keep compiling and calling it unchanged; only callers
    // that need the new capability use Ping. Removing or renaming a method is
    // NOT compatible.
    PingResponse Ping(1: PingRequest req)
}
