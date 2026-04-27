from pydantic import BaseModel
from vyx import ipc


class OrderInput(BaseModel):
    product_id: str
    quantity: int


async def create_order(request: dict) -> dict:
    body = request.get("body", {})
    data = OrderInput(**body)
    return {"order_id": 456, "product": data.product_id, "qty": data.quantity}


async def list_orders(request: dict) -> dict:
    return {"orders": []}


handlers = {
    ("POST", "/api/orders"): create_order,
    ("GET", "/api/orders"): list_orders,
}


if __name__ == "__main__":
    import asyncio
    asyncio.run(ipc.start_worker(handlers))