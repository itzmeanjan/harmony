#!/usr/bin/python3

from python_graphql_client import GraphqlClient
from json import dumps
import asyncio
from sys import argv


def prettyPrint(data):
    print(dumps(data, sort_keys=True, indent=2))


try:

    if len(argv) != 2:
        raise Exception("TxHash not supplied")

    hash = argv[1]
    if not (hash.startswith("0x") and len(hash) == 66):
        raise Exception("Bad txHash")

    client = GraphqlClient(endpoint="ws://localhost:7000/v1/graphql")

    query = """
        subscription watchTx($hash: String!) {
            watchTx(hash: $hash) {
                from
                to
                nonce
                gas
                gasPrice
                queuedFor
                pendingFor
                pool
            }
        }
    """

    print(f'Watching tx : {hash}')

    asyncio.run(client.subscribe(
        query=query, handle=prettyPrint, variables={"hash": hash}))

except Exception as e:
    print(e)
except KeyboardInterrupt:
    print('\nStopping')
