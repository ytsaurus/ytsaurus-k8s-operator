package consts

import "go.ytsaurus.tech/yt/go/schema"

var RawLogsQueueSchema = schema.Schema{
	Columns: []schema.Column{
		{
			Name: "value",
			Type: schema.TypeString,
		},
		{
			Name: "codec",
			Type: schema.TypeString,
		},
		{
			Name: "source_uri",
			Type: schema.TypeString,
		},
		{
			Name:        "meta",
			ComplexType: schema.Optional{Item: schema.TypeAny},
		},
		{
			Name: "$timestamp",
			Type: schema.TypeUint64,
		},
		{
			Name: "$cumulative_data_weight",
			Type: schema.TypeInt64,
		},
	},
}
