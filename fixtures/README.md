# HOWTO: Generate Artifacts

## Crypto material

```bash
$ ./binaries/cryptogen generate --config=./crypto-config.yaml
clark.example.com
```

## Genesis block

```bash
$ FABRIC_CFG_PATH=$PWD ./binaries/configtxgen -profile SystemChannel -outputBlock ./artifacts/system-channel.block -channelID system-channel
2018-10-14 17:26:01.961 UTC [common/configtx/tool] main -> INFO 001 Loading configuration
2018-10-14 17:26:02.050 UTC [common/configtx/tool] doOutputBlock -> INFO 002 Generating genesis block
2018-10-14 17:26:02.053 UTC [common/configtx/tool] doOutputBlock -> INFO 003 Writing genesis block
```

## Channel creation transaction

```bash
$ FABRIC_CFG_PATH=$PWD ./binaries/configtxgen -profile ClarkChannel -outputCreateChannelTx ./artifacts/clark-channel.tx -channelID clark-channel
2018-10-14 17:26:57.257 UTC [common/configtx/tool] main -> INFO 001 Loading configuration
2018-10-14 17:26:57.269 UTC [common/configtx/tool] doOutputChannelCreateTx -> INFO 002 Generating new channel configtx
2018-10-14 17:26:57.272 UTC [common/configtx/tool] doOutputChannelCreateTx -> INFO 003 Writing new channel tx
```