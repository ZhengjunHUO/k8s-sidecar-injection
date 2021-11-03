.PHONY: apply_csr prepare_csr
.DEFAULT_GOAL := apply_csr

cert/server.csr: cert/csr.conf
	@openssl genrsa -out cert/server.key 2048
	@openssl req -config $^ -new -key cert/server.key -out cert/server.csr 

prepare_csr: cert/server.csr 
	@sed -i 's@$${CSR_PLACEHOLDER}@$(shell base64 cert/server.csr | tr -d "\n")@' cert/csr.yaml

apply_csr: prepare_csr
	@kubectl apply -f cert/csr.yaml 	

clean:
	@rm -f cert/server.key cert/server.csr
	@sed -i 's@request:.*@request: $${CSR_PLACEHOLDER}@' cert/csr.yaml
