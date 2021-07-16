package blockchain

import (
	"blockchain_learn/tensor_programming/wallet"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
)

type Transaction struct {
	Id      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

func (tx Transaction) Serialize() []byte {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err.Error())
	}

	return encoded.Bytes()
}

func (tx *Transaction) Hash() []byte {
	var hash [32]byte

	txCopy := *tx
	txCopy.Id = []byte{}

	hash = sha256.Sum256(txCopy.Serialize())

	return hash[:]
}

func (tx *Transaction) SetId() {
	var encoded bytes.Buffer
	var hash [32]byte

	encode := gob.NewEncoder(&encoded)
	err := encode.Encode(tx)
	Handle(err)

	hash = sha256.Sum256(encoded.Bytes())
	tx.Id = hash[:]
}

func CoinbaseTx(to, data string) *Transaction {
	if len(data) == 0 {
		data = fmt.Sprintf("Coins to %s", to)

	}

	txin := TxInput{[]byte{}, -1, nil, []byte(data)}
	txout := NewTXOutput(100, to)

	tx := Transaction{nil, []TxInput{txin}, []TxOutput{*txout}}
	tx.SetId()

	return &tx
}

func NewTransactions(from, to string, amount int, chain *BlockChain) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	wallets, err := wallet.CreateWallets()
	Handle(err)

	w := wallets.GetWallet(from)
	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)

	acc, validOutputs := chain.FindSpendableOutputs(pubKeyHash, amount)

	if acc < amount {
		log.Panic("Error: not enough funds")
	}

	for txid, outs := range validOutputs {
		txId, err := hex.DecodeString(txid)
		Handle(err)

		for _, out := range outs {
			input := TxInput{txId, out, nil, w.PublicKey}
			inputs = append(inputs, input)
		}
	}

	outputs = append(outputs, *NewTXOutput(amount, to))

	if acc > amount {
		outputs = append(outputs, *NewTXOutput(acc-amount, from))
	}

	tx := Transaction{nil, inputs, outputs}
	tx.Id = tx.Hash()
	chain.SignTransaction(&tx, w.PrivateKey)

	return &tx
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].Id) == 0 && tx.Inputs[0].Out == -1
}

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}

	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.Id)].Id == nil {
			log.Panic("previous transaction os not correct")
		}
	}

	txCopy := tx.TrimmedCopy()

	for inId, in := range txCopy.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.Id)]
		txCopy.Inputs[inId].Signature = nil
		txCopy.Inputs[inId].PubKey = prevTX.Outputs[in.Out].PubKeyHash
		txCopy.Id = txCopy.Hash()
		txCopy.Inputs[inId].PubKey = nil

		r, s, err := ecdsa.Sign(rand.Reader, &privKey, txCopy.Id)
		Handle(err)
		signature := append(r.Bytes(), s.Bytes()...)

		tx.Inputs[inId].Signature = signature
	}

}

func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	for _, in := range tx.Inputs {
		inputs = append(inputs, TxInput{in.Id, in.Out, nil, nil})
	}

	for _, out := range tx.Outputs {
		outputs = append(outputs, TxOutput{out.Value, out.PubKeyHash})
	}

	txCopy := Transaction{tx.Id, inputs, outputs}

	return txCopy
}

func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.Id)].Id == nil {
			log.Panic("previous transactions does not exists")
		}
	}

	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()

	for inId, in := range tx.Inputs {
		prevTx := prevTXs[hex.EncodeToString(in.Id)]
		txCopy.Inputs[inId].Signature = nil
		txCopy.Inputs[inId].PubKey = prevTx.Outputs[in.Out].PubKeyHash
		txCopy.Id = txCopy.Hash()
		txCopy.Inputs[inId].PubKey = nil

		r := big.Int{}
		s := big.Int{}
		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])

		rawPubKey := ecdsa.PublicKey{Curve: curve, X: &x, Y: &y}
		if ecdsa.Verify(&rawPubKey, txCopy.Id, &r, &s) == false {
			return false
		}
	}

	return true
}

func (tx Transaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("-- Transaction %x", tx.Id))
	for i, input := range tx.Inputs {
		lines = append(lines, fmt.Sprintf("	Input %d:", i))
		lines = append(lines, fmt.Sprintf("		TXID:		%x", input.Id))
		lines = append(lines, fmt.Sprintf("		Out:		%d", input.Out))
		lines = append(lines, fmt.Sprintf("		Signature:	%x", input.Signature))
		lines = append(lines, fmt.Sprintf("		PubKey:		%x", input.PubKey))
	}

	for i, output := range tx.Outputs {
		lines = append(lines, fmt.Sprintf("	Output %d:", i))
		lines = append(lines, fmt.Sprintf("		Valye:		%d", output.Value))
		lines = append(lines, fmt.Sprintf("		Script:		%x", output.PubKeyHash))
	}

	return strings.Join(lines, "\n")
}
