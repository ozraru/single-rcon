package main

import "context"

func Default(ctx context.Context, conf *ConfigStruct) {
	Check(ctx, conf)
	Install(ctx, conf)
}
