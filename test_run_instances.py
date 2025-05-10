#!/usr/bin/env python3
"""
Simple test script for the RunInstances EC2 function.
"""
# Standard library imports
from typing import Any, Dict
import sys
import os

# Add the test directory to Python path to enable imports
sys.path.append(os.path.abspath("test"))

# Import the RunInstances function from the test.AWS.AWS.ec2 module
from AWS.AWS.ec2.RunInstances import RunInstances
# Import types - these will be available after running the generator
# For now, we'll just use Dict[str, Any] for typing
# from AWS.AWS.ec2.types import RunInstances_body_Type, RunInstances_Result_Type


def test_run_instances():
    """Test the RunInstances function with minimal parameters."""
    # Minimal test parameters
    authKey = "test_auth_key"  # Auth key placeholder
    region = "us-east-1"  # Example AWS region

    # Define the body parameter (required)
    body: RunInstances_body_Type = {
        "ImageId": "ami-12345678",  # Example AMI ID
        "InstanceType": "t2.micro",  # Smallest instance type
        "MinCount": 1,  # Launch at least 1 instance
        "MaxCount": 1,  # Launch at most 1 instance
    }
    """ body: Dict[str, Any] = {
        "ImageId": "ami-12345678",  # Example AMI ID
        "InstanceType": "t2.micro",  # Smallest instance type
        "MinCount": 1,  # Launch at least 1 instance
        "MaxCount": 1,  # Launch at most 1 instance
    }"""

    # Call the function
    result: RunInstances_Result_Type = RunInstances(
        "hi",
        region,
    )
    result = RunInstances(authKey, region, body=body)

    # Print the result
    print("Result:", result)

    # Verify the result
    assert isinstance(result, dict), "Expected dictionary result"

    # For now, we expect an empty dict since it's a stub implementation
    assert result == {}, "Expected empty dict as result"

    print("Test passed!")


if __name__ == "__main__":
    test_run_instances()
