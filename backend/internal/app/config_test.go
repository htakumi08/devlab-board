package app

import "testing"

// テスト内容: HTTPとgRPCのポートについて、既定値と環境変数による上書きが正しく反映されることを確認する。
// 必要な理由: ローカルでは既定値を使い、Dockerなどでは外部設定へ切り替えられるという起動契約を守るため。
func TestLoadConfigServerPorts(t *testing.T) {
	tests := []struct {
		name         string
		httpPort     string
		grpcPort     string
		wantHTTPPort string
		wantGRPCPort string
	}{
		{
			// テスト内容: 環境変数が未設定でも、HTTPとgRPCの既定ポートが設定されることを確認する。
			// 必要な理由: 開発者ごとの事前設定を必須にせず、ローカル起動の再現性を保つため。
			name:         "uses defaults",
			wantHTTPPort: "8080",
			wantGRPCPort: "50051",
		},
		{
			// テスト内容: 環境変数を指定した場合に、既定値ではなく指定値が使われることを確認する。
			// 必要な理由: Dockerやデプロイ先の制約に応じて、コード変更なしで待受ポートを変更するため。
			name:         "uses environment values",
			httpPort:     "18080",
			grpcPort:     "15051",
			wantHTTPPort: "18080",
			wantGRPCPort: "15051",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PORT", tt.httpPort)
			t.Setenv("GRPC_PORT", tt.grpcPort)

			config := LoadConfig()

			if config.HTTPPort != tt.wantHTTPPort {
				t.Errorf("HTTPPort = %q; want %q", config.HTTPPort, tt.wantHTTPPort)
			}
			if config.GRPCPort != tt.wantGRPCPort {
				t.Errorf("GRPCPort = %q; want %q", config.GRPCPort, tt.wantGRPCPort)
			}
		})
	}
}
