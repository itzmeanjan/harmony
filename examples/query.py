#!/usr/bin/python3

from python_graphql_client import GraphqlClient
from json import dumps

client = GraphqlClient(endpoint="http://localhost:7000/v1/graphql")

query = """
    query {
        pendingForMoreThan(x: "10s") {
            from
            to
  	        gasPrice
            nonce
        }
    }
"""

data = client.execute(query=query)
print(dumps(data, sort_keys=True, indent=2))
