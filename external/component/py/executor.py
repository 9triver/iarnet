from actorc.executor import Executor
from argparse import ArgumentParser


def parse_args():
    parser = ArgumentParser()
    parser.add_argument("--conn-id", type=str, required=True)
    parser.add_argument("--remote", type=str, required=True)
    return parser.parse_args()


def main():
    args = parse_args()
    executor = Executor(args.conn_id)
    executor.serve(args.remote)


if __name__ == "__main__":
    main()
