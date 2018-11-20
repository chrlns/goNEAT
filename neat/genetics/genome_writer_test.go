package genetics

import (
	"testing"
	"bytes"
	"github.com/yaricom/goNEAT/neat"
	"fmt"
	"strings"
	"bufio"
	"github.com/yaricom/goNEAT/neat/network"
	"reflect"
)

func TestPlainGenomeWriter_WriteTrait(t *testing.T) {
	params := []float64{
		0.40227575878298616, 0.0, 0.0, 0.0, 0.0, 0.3245553261200018, 0.0, 0.12248956525856575,
	}
	trait_id := 2
	trait := neat.NewTrait()
	trait.Id = trait_id
	trait.Params = params

	trait_str := fmt.Sprintf("%d %g %g %g %g %g %g %g %g",
		trait_id, params[0], params[1], params[2], params[3], params[4], params[5], params[6], params[7])

	out_buffer := bytes.NewBufferString("")
	wr := plainGenomeWriter{w:bufio.NewWriter(out_buffer)}
	err := wr.writeTrait(trait)
	if err != nil {
		t.Error(err)
		return
	}
	wr.w.Flush()

	out_str := strings.TrimSpace(out_buffer.String())
	if trait_str != out_str {
		t.Errorf("Wrong trait serialization\n[%s]\n[%s]", trait_str, out_str)
	}
}

// Tests NNode serialization
func TestPlainGenomeWriter_WriteNetworkNode(t *testing.T) {
	node_id, trait_id, ntype, neuron_type := 1, 10, network.SensorNode, network.InputNeuron
	node_str := fmt.Sprintf("%d %d %d %d", node_id, trait_id, ntype, neuron_type)
	trait := neat.NewTrait()
	trait.Id = 10

	node := network.NewNNode(node_id, neuron_type)
	node.Trait = trait
	out_buffer := bytes.NewBufferString("")

	wr := plainGenomeWriter{w:bufio.NewWriter(out_buffer)}
	err := wr.writeNetworkNode(node)
	if err != nil {
		t.Error(err)
		return
	}
	wr.w.Flush()

	out_str := out_buffer.String()

	if out_str != node_str {
		t.Errorf("Node serialization failed. Expected: %s, but found %s", node_str, out_str)
	}
}

func TestPlainGenomeWriter_WriteConnectionGene(t *testing.T) {
	// gene  1 1 4 1.1983046913458986 0 1.0 1.1983046913458986 0
	traitId, inNodeId, outNodeId, innov_num := 1, 1, 4, int64(1)
	weight, mut_num := 1.1983046913458986, 1.1983046913458986
	recurrent, enabled := false, false
	gene_str := fmt.Sprintf("%d %d %d %g %t %d %g %t",
		traitId, inNodeId, outNodeId, weight, recurrent, innov_num, mut_num, enabled)

	trait := neat.NewTrait()
	trait.Id = traitId
	gene := NewGeneWithTrait(trait, weight, network.NewNNode(1, network.InputNeuron),
		network.NewNNode(4, network.HiddenNeuron), recurrent, innov_num, mut_num)
	gene.IsEnabled = enabled

	out_buf := bytes.NewBufferString("")

	wr := plainGenomeWriter{w:bufio.NewWriter(out_buf)}
	err := wr.writeConnectionGene(gene)
	if err != nil {
		t.Error(err)
		return
	}
	wr.w.Flush()

	out_str := out_buf.String()
	if gene_str != out_str {
		t.Errorf("Wrong Gene serialization\n[%s]\n[%s]", gene_str, out_str)
	}
}

func TestPlainGenomeWriter_WriteGenome(t *testing.T) {
	gnome := buildTestGenome(1)
	out_buf := bytes.NewBufferString("")
	wr, err := NewGenomeWriter(bufio.NewWriter(out_buf), PlainGenomeEncoding)
	if err == nil {
		err = wr.WriteGenome(gnome)
	}
	if err != nil {
		t.Error(err)
		return
	}

	_, g_str_r, err_g := bufio.ScanLines([]byte(gnome_str), true)
	_, o_str_r, err_o := bufio.ScanLines(out_buf.Bytes(), true)
	if err_g != nil || err_o != nil {
		t.Error("Failed to parse strings", err_o, err_g)
	}
	for i, gsr := range g_str_r {
		if gsr != o_str_r[i] {
			t.Error("Lines mismatch", gsr, o_str_r[i])
		}
	}
}

func TestYamlGenomeWriter_WriteGenome(t *testing.T) {
	gnome := buildTestGenome(1)

	// encode genome
	out_buf := bytes.NewBufferString("")
	wr, err := NewGenomeWriter(bufio.NewWriter(out_buf), YAMLGenomeEncoding)
	if err == nil {
		err = wr.WriteGenome(gnome)
	}
	if err != nil {
		t.Error(err)
		return
	}
	//t.Log(out_buf.String())

	// decode genome and compare
	enc := yamlGenomeReader{r:bufio.NewReader(bytes.NewBuffer(out_buf.Bytes()))}
	gnome_enc, err := enc.Read()
	if err != nil {
		t.Error(err)
		return
	}

	if gnome.Id != gnome_enc.Id {
		t.Error("gnome.Id != gnome_enc.Id", gnome.Id, gnome_enc.Id)
	}
	if len(gnome.Genes) != len(gnome_enc.Genes) {
		t.Error("len(gnome.Genes) != len(gnome_enc.Genes)", len(gnome.Genes), len(gnome_enc.Genes))
	}
	for i, g := range gnome.Genes {
		og := gnome_enc.Genes[i]
		if !g.Link.IsEqualGenetically(og.Link) {
			t.Error("!g.Link.IsEqualGenetically(og.Link) at:", i)
		}
		if g.IsEnabled != og.IsEnabled {
			t.Error("g.IsEnabled != og.IsEnabled at:", i)
		}
		if g.MutationNum != og.MutationNum {
			t.Error("g.MutationNum != og.MutationNum at:", i)
		}
		if g.InnovationNum != og.InnovationNum {
			t.Error("g.InnovationNum != og.InnovationNum at:", i)
		}
	}


	if len(gnome.Nodes) != len(gnome_enc.Nodes) {
		t.Error("len(gnome.Nodes) != len(gnome_enc.Nodes)", len(gnome.Nodes), len(gnome_enc.Nodes))
	}
	for i, n := range gnome.Nodes {
		nd := gnome_enc.Nodes[i]
		if n.Id != nd.Id {
			t.Error("n.Id != nd.Id at:", i)
		}
		if n.ActivationType != nd.ActivationType {
			t.Error("n.ActivationType != nd.ActivationType at:", i)
		}
		if n.NeuronType != nd.NeuronType {
			t.Error("n.NeuronType != nd.NeuronType at:", i)
		}
	}

	if len(gnome.Traits) != len(gnome_enc.Traits) {
		t.Error("len(gnome.Traits) != len(gnome_enc.Traits)", len(gnome.Traits), len(gnome_enc.Traits))
	}
	for i, tr := range gnome.Traits {
		etr := gnome_enc.Traits[i]
		if tr.Id != etr.Id {
			t.Error("tr.Id != etr.Id at:", i)
		}
		if !reflect.DeepEqual(tr.Params, etr.Params) {
			t.Error("!reflect.DeepEqual(tr.Params, etr.Params) at:", i)
		}
	}
}