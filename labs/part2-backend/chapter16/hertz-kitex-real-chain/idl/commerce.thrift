namespace go commerce

struct User {
  1: string id
  2: string name
}

struct Product {
  1: string id
  2: string name
  3: i64 price_cents
}

struct OrderItem {
  1: string product_id
  2: string product_name
  3: i64 unit_price_cents
  4: i64 quantity
}

struct Order {
  1: string id
  2: string user_id
  3: list<OrderItem> items
  4: i64 total_cents
}

struct UserOrdersRequest {
  1: string user_id
}

struct UserOrdersResponse {
  1: User user
  2: list<Order> orders
}

struct ProductRequest {
  1: string product_id
}

struct ListOrdersRequest {
  1: string user_id
}

struct ListOrdersResponse {
  1: list<Order> orders
}

struct PingRequest {}
struct PingResponse {
  1: string ok
}

service ProductService {
  Product GetProduct(1: ProductRequest req)
  PingResponse Ping(1: PingRequest req)
}

service OrderService {
  ListOrdersResponse ListOrders(1: ListOrdersRequest req)
  PingResponse Ping(1: PingRequest req)
}

service UserService {
  UserOrdersResponse GetUserOrders(1: UserOrdersRequest req)
  PingResponse Ping(1: PingRequest req)
}
