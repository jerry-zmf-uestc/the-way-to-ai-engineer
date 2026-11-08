namespace go api

struct EchoRequest {
    1: string message
}

struct EchoResponse {
    1: string message
    2: i32 length
}

struct AddRequest {
    1: i64 first
    2: i64 second
}

struct AddResponse {
    1: i64 sum
}

service EchoService {
    EchoResponse Echo(1: EchoRequest req)
    AddResponse Add(1: AddRequest req)
}
