from pydantic import BaseModel
from vyx import ipc


class OrderInput(BaseModel):
    product_id: str
    quantity: int


# @Route(POST /api/orders)
# @Validate(pydantic)
# @Auth(roles: ["user"])
async def create_order(request: dict) -> dict:
    body = request.get("body", {})
    data = OrderInput(**body)
    return {"order_id": 456, "product": data.product_id, "qty": data.quantity}


# @Route(GET /api/orders)
# @Auth(roles: ["user", "admin"])
async def list_orders(request: dict) -> dict:
    return []


# @Route(GET /api/health)
def health_check(request: dict) -> dict:
    return {"status": "ok"}


handlers = {
    ("POST", "/api/orders"): create_order,
    ("GET", "/api/orders"): list_orders,
}


if __name__ == "__main__":
    import asyncio
    asyncio.run(ipc.start_worker(handlers))