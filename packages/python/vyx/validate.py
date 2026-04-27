from typing import Any


def validate_pydantic(model_cls: type, data: Any):
    """Validate data against a Pydantic model.

    Args:
        model_cls: Pydantic BaseModel subclass
        data: Data to validate

    Returns:
        Validated model instance

    Raises:
        ValidationError: If validation fails
    """
    if isinstance(data, dict):
        return model_cls(**data)
    return model_cls.model_validate(data)


def get_validator(validate_type: str):
    """Get validator function for the given validate type.

    Args:
        validate_type: One of "pydantic", "jsonschema", "none"

    Returns:
        Validator function or None for "none"
    """
    if validate_type == "pydantic":
        return validate_pydantic
    return None


class ValidationError(Exception):
    def __init__(self, errors: list):
        self.errors = errors
        super().__init__(str(errors))
