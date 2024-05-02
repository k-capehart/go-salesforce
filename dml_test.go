package salesforce

import (
	"net/http"
	"reflect"
	"testing"
)

func Test_convertToMap(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	type args struct {
		obj any
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]any
		wantErr bool
	}{
		{
			name: "convert_account_to_map",
			args: args{obj: account{
				Id:   "1234",
				Name: "test account",
			}},
			want: map[string]any{
				"Id":   "1234",
				"Name": "test account",
			},
			wantErr: false,
		},
		{
			name:    "convert_fail",
			args:    args{obj: 1},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToMap(tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToMap() = %v, want %v", got, tt.want)
			}
		})
	}

}

func Test_convertToSliceOfMaps(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	type args struct {
		obj any
	}
	tests := []struct {
		name    string
		args    args
		want    []map[string]any
		wantErr bool
	}{
		{
			name: "convert_account_slice_to_slice_of_maps",
			args: args{obj: []account{
				{
					Id:   "1234",
					Name: "test account 1",
				},
				{
					Id:   "5678",
					Name: "test account 2",
				},
			}},
			want: []map[string]any{
				{
					"Id":   "1234",
					"Name": "test account 1",
				},
				{
					"Id":   "5678",
					"Name": "test account 2",
				},
			},
			wantErr: false,
		},
		{
			name:    "convert_fail",
			args:    args{obj: 1},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToSliceOfMaps(tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToSliceOfMaps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToSliceOfMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doBatchedRequestsForCollection(t *testing.T) {
	server, sfAuth := setupTestServer([]salesforceError{{Success: true}}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth      authentication
		method    string
		url       string
		batchSize int
		recordMap []map[string]any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "single_record",
			args: args{
				auth:      sfAuth,
				method:    http.MethodPost,
				url:       "",
				batchSize: 200,
				recordMap: []map[string]any{
					{
						"Name": "test record 1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple batches",
			args: args{
				auth:      sfAuth,
				method:    http.MethodPost,
				url:       "",
				batchSize: 1,
				recordMap: []map[string]any{
					{
						"Name": "test record 1",
					},
					{
						"Name": "test record 2",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := doBatchedRequestsForCollection(tt.args.auth, tt.args.method, tt.args.url, tt.args.batchSize, tt.args.recordMap); (err != nil) != tt.wantErr {
				t.Errorf("doBatchedRequestsForCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
