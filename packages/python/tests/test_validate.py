"""Tests for validate.py module."""
import pytest
from pydantic import BaseModel

from vyx.validate import validate_pydantic, get_validator, ValidationError


class TestModel(BaseModel):
    """Test pydantic model."""
    name: str
    age: int


class TestValidatePydantic:
    """Test validate_pydantic function."""

    def test_validate_dict(self):
        """Test validating a dictionary."""
        data = {"name": "John", "age": 30}
        result = validate_pydantic(TestModel, data)
        
        assert result.name == "John"
        assert result.age == 30
        assert isinstance(result, TestModel)

    def test_validate_instance(self):
        """Test validating an instance."""
        instance = TestModel(name="Jane", age=25)
        result = validate_pydantic(TestModel, instance)
        
        assert result.name == "Jane"
        assert result.age == 25

    def test_validate_invalid_data(self):
        """Test validating invalid data."""
        data = {"name": "John"}  # Missing age
        
        with pytest.raises(Exception) as exc_info:
            validate_pydantic(TestModel, data)
        
        # Should raise some kind of validation error
        assert exc_info.value is not None

    def test_validate_invalid_type(self):
        """Test validating data with wrong types."""
        data = {"name": "John", "age": "not_a_number"}
        
        with pytest.raises(Exception):
            validate_pydantic(TestModel, data)


class TestGetValidator:
    """Test get_validator function."""

    def test_get_pydantic_validator(self):
        """Test getting pydantic validator."""
        validator = get_validator("pydantic")
        
        assert validator is not None
        assert callable(validator)
        # Verify it's the validate_pydantic function
        assert validator == validate_pydantic

    def test_get_jsonschema_validator(self):
        """Test getting jsonschema validator."""
        validator = get_validator("jsonschema")
        
        # Should return None for jsonschema (not implemented)
        assert validator is None

    def test_get_none_validator(self):
        """Test getting 'none' validator."""
        validator = get_validator("none")
        
        assert validator is None

    def test_get_invalid_validator(self):
        """Test getting invalid validator type."""
        validator = get_validator("invalid_type")
        
        assert validator is None


class TestValidationError:
    """Test ValidationError exception."""

    def test_validation_error_creation(self):
        """Test creating ValidationError."""
        errors = [{"field": "name", "message": "required"}]
        exc = ValidationError(errors)
        
        assert exc.errors == errors
        assert str(errors) in str(exc)

    def test_validation_error_empty(self):
        """Test creating ValidationError with empty errors."""
        exc = ValidationError([])
        
        assert exc.errors == []
        assert str(exc) != ""

    def test_validation_error_inheritance(self):
        """Test that ValidationError inherits from Exception."""
        assert issubclass(ValidationError, Exception)
        
        exc = ValidationError([{"error": "test"}])
        assert isinstance(exc, Exception)
