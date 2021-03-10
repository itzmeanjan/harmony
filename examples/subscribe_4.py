#!/usr/bin/python3

from python_graphql_client import GraphqlClient
from json import dumps
import asyncio

def prettyPrint(data):
        print(dumps(data, sort_keys=True, indent=2))

try:
    client = GraphqlClient(endpoint="ws://localhost:7000/v1/graphql")

    query = """
        subscription {
            newConfirmedTxFrom(address: "0x63ec5767F54F6943750A70eB6117EA2D9Ca77313") {
                from
                to
                nonce
                gasPrice
                pendingFor
            }
        }
    """

    print('Listening for any new tx, leaving pending pool, from `0x63ec5767F54F6943750A70eB6117EA2D9Ca77313`')

    asyncio.run(client.subscribe(query=query, handle=prettyPrint))

except Exception as e:
    print(e)
except KeyboardInterrupt:
    print('\nStopping')
