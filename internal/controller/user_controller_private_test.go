package controller

import (
	"context"
	"encoding/base64"
	"regexp"
	"testing"

	operatorv1alpha1 "github.com/sickhub/mailu-operator/api/v1alpha1"
	"github.com/sickhub/mailu-operator/pkg/mailu"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUserReconciler_getRawUserPassword(t *testing.T) {
	type fields struct {
		Client    client.Client
		Scheme    *runtime.Scheme
		ApiURL    string
		ApiToken  string
		ApiClient *mailu.Client
	}
	type args struct {
		ctx  context.Context
		user *operatorv1alpha1.User
	}

	k8sClient := fake.NewClientBuilder().Build()
	k8sClient.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	passworB64 := make([]byte, base64.StdEncoding.EncodedLen(8))
	base64.StdEncoding.Encode(passworB64, []byte("password"))
	k8sClient.Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": passworB64,
		},
	})

	tests := []struct {
		name      string
		fields    fields
		args      args
		wantRegex string
		wantErr   bool
	}{
		{
			name: "test generated password",
			fields: fields{
				Client:    nil,
				Scheme:    nil,
				ApiURL:    "",
				ApiToken:  "",
				ApiClient: nil,
			},
			args: args{
				ctx: context.Background(),
				user: &operatorv1alpha1.User{
					Spec: operatorv1alpha1.UserSpec{
						Name:   "test",
						Domain: "test.com",
					},
				},
			},
			wantRegex: ".{20}",
			wantErr:   false,
		},
		{
			name: "test missing secret",
			fields: fields{
				Client:    k8sClient,
				Scheme:    nil,
				ApiURL:    "",
				ApiToken:  "",
				ApiClient: nil,
			},
			args: args{
				ctx: context.Background(),
				user: &operatorv1alpha1.User{
					Spec: operatorv1alpha1.UserSpec{
						Name:           "test",
						Domain:         "test.com",
						PasswordSecret: "foo",
						PasswordKey:    "bar",
					},
				},
			},
			wantRegex: "",
			wantErr:   true,
		},
		{
			name: "test existing secret",
			fields: fields{
				Client:    k8sClient,
				Scheme:    nil,
				ApiURL:    "",
				ApiToken:  "",
				ApiClient: nil,
			},
			args: args{
				ctx: context.Background(),
				user: &operatorv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testUser",
						Namespace: "default",
					},
					Spec: operatorv1alpha1.UserSpec{
						Name:           "test",
						Domain:         "test.com",
						PasswordSecret: "secret",
						PasswordKey:    "password",
					},
				},
			},
			wantRegex: "(password)",
			wantErr:   false,
		},
		{
			name: "test existing secret with wrong key",
			fields: fields{
				Client:    k8sClient,
				Scheme:    nil,
				ApiURL:    "",
				ApiToken:  "",
				ApiClient: nil,
			},
			args: args{
				ctx: context.Background(),
				user: &operatorv1alpha1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testUser",
						Namespace: "default",
					},
					Spec: operatorv1alpha1.UserSpec{
						Name:           "test",
						Domain:         "test.com",
						PasswordSecret: "secret",
						PasswordKey:    "non-existent",
					},
				},
			},
			wantRegex: "",
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &UserReconciler{
				Client:    tt.fields.Client,
				Scheme:    tt.fields.Scheme,
				ApiURL:    tt.fields.ApiURL,
				ApiToken:  tt.fields.ApiToken,
				ApiClient: tt.fields.ApiClient,
			}
			got, err := r.getRawUserPassword(tt.args.ctx, tt.args.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("getRawUserPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			reg, err := regexp.Compile(tt.wantRegex)
			if err != nil {
				t.Fatal(err)
			}
			if !reg.MatchString(got) {
				t.Errorf("getRawUserPassword() %v to match regex %v", got, tt.wantRegex)
			}
		})
	}
}
