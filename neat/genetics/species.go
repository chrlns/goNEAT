package genetics

import (
	"github.com/yaricom/goNEAT/neat"
	"sort"
	"math"
	"fmt"
	"errors"
	"math/rand"
	//"github.com/yaricom/goNEAT/neat/network"
	"io"
)

// A Species is a group of similar Organisms.
// Reproduction takes place mostly within a single species, so that compatible organisms can mate.
type Species struct {
	// The ID
	Id                   int;
	// The age of the Species
	Age                  int
	// The average fitness of the Species
	AvgFitness           float64
	// The maximal fitness of the Species
	MaxFitness           float64
	// The maximal fitness it ever had
	MaxFitnessEver       float64
	// How many child expected
	ExpectedOffspring    int

	// Is it novel
	IsNovel              bool

	// The organisms in the Species
	Organisms            []*Organism
	// If this is too long ago, the Species will goes extinct
	AgeOfLastImprovement int
}

// Construct new species with specified ID
func NewSpecies(id int) *Species {
	return newSpecies(id)
}

// Allows the creation of a Species that won't age (a novel one). This protects new Species from aging
// inside their first generation
func NewSpeciesNovel(id int, novel bool) *Species {
	s := newSpecies(id)
	s.IsNovel = novel

	return s
}

// The private default constructor
func newSpecies(id int) *Species {
	return &Species{
		Id:id,
		Age:1,
		Organisms:make([]*Organism, 0),
	}
}

// Writes species to the specified writer
func (s *Species) Write(w io.Writer) {
	// Print a comment on the Species info
	fmt.Fprintf(w, "/* Species #%d : (Size %d) (AF %.3f) (Age %d)  */\n",
		s.Id, len(s.Organisms), s.AvgFitness, s.Age)

	// Print all the Organisms' Genomes to the outFile
	for _, org := range s.Organisms {
		fmt.Fprintf(w, "/* Organism #%d Fitness: %.3f Error: %.3f */\n",
			org.GNome.Id, org.Fitness, org.Error)
		if org.IsWinner {
			fmt.Fprintf(w, "/* ##------$ WINNER %d SPECIES # %d $------## */\n",
				org.GNome.Id, s.Id)
		}
		org.GNome.Write(w)
	}
}


// Adds new Organism to the group related to this Species
func (s *Species) addOrganism(o *Organism) {
	s.Organisms = append(s.Organisms, o)
}
// Removes an organism from Species
func (s *Species) removeOrganism(org *Organism) (bool, error) {
	orgs := make([]*Organism, 0)
	for _, o := range s.Organisms {
		if o != org {
			orgs = append(orgs, o)
		}
	}
	if len(orgs) != len(s.Organisms) - 1 {
		return false, errors.New("ALERT: Attempt to remove nonexistent Organism from Species")
	} else {
		s.Organisms = orgs
		return true, nil
	}
}

// Can change the fitness of the organisms in the Species to be higher for very new species (to protect them).
// Divides the fitness by the size of the Species, so that fitness is "shared" by the species.
func (s *Species) adjustFitness(conf *neat.NeatContext) {
	age_debt := (s.Age - s.AgeOfLastImprovement + 1) - conf.DropOffAge
	if age_debt == 0 {
		age_debt = 1
	}

	for _, org := range s.Organisms {
		// Remember the original fitness before it gets modified
		org.OriginalFitness = org.Fitness

		// Make fitness decrease after a stagnation point dropoff_age
		// Added as if to keep species pristine until the dropoff point
		if age_debt >= 1 {
			// Extreme penalty for a long period of stagnation (divide fitness by 100)
			org.Fitness = org.Fitness * 0.01
		}

		// Give a fitness boost up to some young age (niching)
		// The age_significance parameter is a system parameter
		// if it is 1, then young species get no fitness boost
		if s.Age <= 10 {
			org.Fitness = org.Fitness * conf.AgeSignificance
		}
		// Do not allow negative fitness
		if org.Fitness < 0.0 {
			org.Fitness = 0.0001
		}

		// Share fitness with the species
		org.Fitness = org.Fitness / float64(len(s.Organisms))
	}

	// Sort the population (most fit first) and mark for death those after : survival_thresh * pop_size
	sort.Sort(ByFitness(s.Organisms))

	// Update age_of_last_improvement here
	if s.Organisms[0].OriginalFitness > s.MaxFitnessEver {
		s.AgeOfLastImprovement = s.Age
		s.MaxFitnessEver = s.Organisms[0].OriginalFitness
	}

	// Decide how many get to reproduce based on survival_thresh * pop_size
	// Adding 1.0 ensures that at least one will survive
	num_parents := int(math.Floor(conf.SurvivalThresh * float64(len(s.Organisms)) + 1.0))

	// Mark for death those who are ranked too low to be parents
	s.Organisms[0].IsChampion = true // Mark the champ as such
	for c := num_parents; c < len(s.Organisms); c++ {
		s.Organisms[c].ToEliminate = true
	}
}

// Computes average species fitness
func (s *Species) ComputeAvgFitness() float64 {
	total := 0.0
	for _, o := range s.Organisms {
		total += o.Fitness
	}
	s.AvgFitness = total / float64(len(s.Organisms))
	return s.AvgFitness
}

// Computes maximal fitness of species
func (s *Species) ComputeMaxFitness() float64 {
	max := 0.0
	for _, o := range s.Organisms {
		if o.Fitness > max {
			max = o.Fitness
		}
	}
	s.MaxFitness = max
	return s.MaxFitness
}

// Returns first organism or nil
func (s *Species) firstOrganism() *Organism {
	if len(s.Organisms) > 0 {
		return s.Organisms[0]
	} else {
		return nil
	}
}

// Compute the collective offspring the entire species (the sum of all organism's offspring) is assigned.
// The skim is fractional offspring left over from a previous species that was counted. These fractional parts are
// kept until they add up to 1.
// Returns fractional offspring left after computation (skim).
func (s *Species) countOffspring(skim float64) float64 {
	var e_o_intpart int  // The floor of an organism's expected offspring
	var e_o_fracpart float64 // Expected offspring fractional part
	var skim_intpart float64  // The whole offspring in the skim

	s.ExpectedOffspring = 0
	for _, o := range s.Organisms {
		e_o_intpart = int(math.Floor(o.ExpectedOffspring))
		e_o_fracpart = math.Mod(o.ExpectedOffspring, 1.0)

		s.ExpectedOffspring += e_o_intpart

		// Skim off the fractional offspring
		skim += e_o_fracpart

		if skim >= 1.0 {
			skim_intpart = math.Floor(skim)
			s.ExpectedOffspring += int(skim_intpart)
			skim -= skim_intpart
		}
	}
	return skim
}

// Compute generations since last improvement
func (s *Species) lastImproved() int {
	return s.Age - s.AgeOfLastImprovement
}

// Returns size of this Species, i.e. number of Organisms belonging to it
func (s *Species) size() int {
	return len(s.Organisms)
}

// Returns Organism - champion among others (best fitness)
func (s *Species) findChampion() *Organism {
	best_fitness := 0.0
	var champion *Organism
	for _, o := range s.Organisms {
		if o.Fitness > best_fitness {
			best_fitness = o.Fitness
			champion = o
		}
	}
	return champion
}

// Perform mating and mutation to form next generation. The sorted_species is ordered to have best species in the beginning.
func (s *Species) reproduce(generation int, pop *Population, sorted_species []*Species, context *neat.NeatContext) (bool, error) {
	//Check for a mistake
	if s.ExpectedOffspring > 0 && len(s.Organisms) == 0 {
		return false, errors.New("SPECIES: ERROR: ATTEMPT TO REPRODUCE OUT OF EMPTY SPECIES")
	}

	// The number of Organisms in the old generation
	pool_size := len(s.Organisms)
	// The champion of the 'this' specie is the first element of the specie;
	the_champ := s.Organisms[0]

	// Parent Organisms and new Organism
	var baby *Organism

	// Flag the preservation of the champion
	champ_clone_done := false

	// Create the designated number of offspring for the Species one at a time
	for count := 0; count < s.ExpectedOffspring; count++ {
		neat.DebugLog(fmt.Sprintf("\nSPECIES: Offspring #%d from %d, (species: %d)",
			count, s.ExpectedOffspring, s.Id))

		mut_struct_baby, mate_baby := false, false

		// Debug Trap
		if s.ExpectedOffspring > context.PopSize {
			neat.WarnLog(fmt.Sprintf("SPECIES: ALERT: Species [%d] expected offspring: %d exceeds population size limit: %d\n",
				s.Id, s.ExpectedOffspring, context.PopSize))
		}

		if the_champ.SuperChampOffspring > 0 {
			neat.DebugLog("SPECIES: Reproduce super champion")

			// If we have a super_champ (Population champion), finish off some special clones
			mom := the_champ;
			new_genome := mom.GNome.duplicate(count)

			// Most superchamp offspring will have their connection weights mutated only
			// The last offspring will be an exact duplicate of this super_champ
			// Note: Superchamp offspring only occur with stolen babies!
			//      Settings used for published experiments did not use this
			if the_champ.SuperChampOffspring > 1 {
				if rand.Float64() < 0.8 || context.MutateAddLinkProb == 0.0 {
					// Make sure no links get added when the system has link adding disabled
					new_genome.mutateLinkWeights(context.WeightMutPower, 1.0, GAUSSIAN)
				} else {
					// Sometimes we add a link to a superchamp
					new_genome.genesis(generation)
					_, err := new_genome.mutateAddLink(pop, context)
					if err != nil {
						return false, err
					}
					mut_struct_baby = true;
				}
			}

			// Create the new baby organism
			baby = NewOrganism(0.0, new_genome, generation)

			if the_champ.SuperChampOffspring == 1 {
				if the_champ.IsPopulationChampion {
					baby.IsPopulationChampionChild = true
					baby.highestFitness = mom.OriginalFitness
				}
			}

			the_champ.SuperChampOffspring--
		} else if !champ_clone_done && s.ExpectedOffspring > 5 {
			neat.DebugLog("SPECIES: Clone species champion")

			// If we have a Species champion, just clone it
			mom := the_champ // Mom is the champ
			new_genome := mom.GNome.duplicate(count)
			// Baby is just like mommy
			champ_clone_done = true

			// Create the new baby organism
			baby = NewOrganism(0.0, new_genome, generation)

		} else if rand.Float64() < context.MutateOnlyProb || pool_size == 1 {
			neat.DebugLog("SPECIES: Reproduce by applying random mutation:")

			// Apply mutations
			org_num := rand.Int31n(int32(pool_size)) // select random mom
			mom := s.Organisms[org_num]
			new_genome := mom.GNome.duplicate(count)

			// Do the mutation depending on probabilities of various mutations
			if rand.Float64() < context.MutateAddNodeProb {
				neat.DebugLog("SPECIES: ---> mutateAddNode")

				// Mutate add node
				_, err := new_genome.mutateAddNode(pop, context)
				if err != nil {
					return false, err
				}
				mut_struct_baby = true
			} else if rand.Float64() < context.MutateAddLinkProb {
				neat.DebugLog("SPECIES: ---> mutateAddLink")

				// Mutate add link
				new_genome.genesis(generation)
				_, err := new_genome.mutateAddLink(pop, context)
				if err != nil {
					return false, err
				}
				mut_struct_baby = true
			} else if rand.Float64() < context.MutateConnectSensors {
				neat.DebugLog("SPECIES: ---> mutateConnectSensors")
				link_added, err := new_genome.mutateConnectSensors(pop, context)
				if err != nil {
					return false, err
				}
				mut_struct_baby = link_added
			}

			if !mut_struct_baby {
				neat.DebugLog("SPECIES: ---> mutateAllNonstructural")

				// If we didn't do a structural mutation, we do the other kinds
				_, err := new_genome.mutateAllNonstructural(context)
				if err != nil {
					return false, err
				}
			}

			// Create the new baby organism
			baby = NewOrganism(0.0, new_genome, generation);
		} else {
			neat.DebugLog("SPECIES: Reproduce by mating:")

			// Otherwise we should mate
			org_num := rand.Int31n(int32(pool_size)) // select random mom
			mom := s.Organisms[org_num]

			// Choose random dad
			var dad *Organism
			if rand.Float64() > context.InterspeciesMateRate {
				neat.DebugLog("SPECIES: ---> mate within species")

				// Mate within Species
				org_num = rand.Int31n(int32(pool_size))
				dad = s.Organisms[org_num]
			} else {
				neat.DebugLog("SPECIES: ---> mate outside species")

				// Mate outside Species
				rand_species := s

				// Select a random species
				giveup := 0
				for ; rand_species == s && giveup < 5; {
					// Choose a random species tending towards better species
					rand_mult := rand.Float64() / 4.0
					// This tends to select better species
					rand_species_num := int(math.Floor(rand_mult * float64(len(sorted_species))))
					rand_species = sorted_species[rand_species_num]

					giveup++
				}
				dad = rand_species.Organisms[0]
			}

			// Perform mating based on probabilities of different mating types
			var new_genome *Genome
			var err error
			if rand.Float64() < context.MateMultipointProb {
				neat.DebugLog("SPECIES: ------> mateMultipoint")

				// mate multipoint baby
				new_genome, err = mom.GNome.mateMultipoint(dad.GNome, count, mom.OriginalFitness, dad.OriginalFitness)
				if err != nil {
					return false, err
				}
			} else if rand.Float64() < context.MateMultipointAvgProb / (context.MateMultipointAvgProb + context.MateSinglepointProb) {
				neat.DebugLog("SPECIES: ------> mateMultipointAvg")

				// mate multipoint_avg baby
				new_genome, err = mom.GNome.mateMultipointAvg(dad.GNome, count, mom.OriginalFitness, dad.OriginalFitness)
				if err != nil {
					return false, err
				}
			} else {
				neat.DebugLog("SPECIES: ------> mateSinglepoint")

				new_genome, err = mom.GNome.mateSinglepoint(dad.GNome, count)
				if err != nil {
					return false, err
				}
			}

			mate_baby = true

			// Determine whether to mutate the baby's Genome
			// This is done randomly or if the mom and dad are the same organism
			if rand.Float64() > context.MateOnlyProb ||
				dad.GNome.Id == mom.GNome.Id ||
				dad.GNome.compatibility(mom.GNome, context) == 0.0 {
				neat.DebugLog("SPECIES: ------> Mutatte baby genome:")

				// Do the mutation depending on probabilities of  various mutations
				if rand.Float64() < context.MutateAddNodeProb {
					neat.DebugLog("SPECIES: ---------> mutateAddNode")

					// mutate_add_node
					_, err = new_genome.mutateAddNode(pop, context)
					if err != nil {
						return false, err
					}
					mut_struct_baby = true
				} else if rand.Float64() < context.MutateAddLinkProb {
					neat.DebugLog("SPECIES: ---------> mutateAddLink")

					// mutate_add_link
					new_genome.genesis(generation)
					_, err = new_genome.mutateAddLink(pop, context)
					if err != nil {
						return false, err
					}
					mut_struct_baby = true
				} else if rand.Float64() < context.MutateConnectSensors {
					neat.DebugLog("SPECIES: ---> mutateConnectSensors")
					link_added, err := new_genome.mutateConnectSensors(pop, context)
					if err != nil {
						return false, err
					}
					mut_struct_baby = link_added
				}

				if !mut_struct_baby {
					neat.DebugLog("SPECIES: ---> mutateAllNonstructural")

					// If we didn't do a structural mutation, we do the other kinds
					_, err := new_genome.mutateAllNonstructural(context)
					if err != nil {
						return false, err
					}
				}
			}
			// Create the new baby organism
			baby = NewOrganism(0.0, new_genome, generation)
		} // end else

		// Add the baby to its proper Species
		// If it doesn't fit a Species, create a new one
		baby.mutationStructBaby = mut_struct_baby
		baby.mateBaby = mate_baby

		if len(pop.Species) == 0 {
			// Create the first species
			createFirstSpecies(pop, baby)
		} else {
			if context.CompatThreshold == 0 {
				return false, errors.New("SPECIES: compatibility thershold is set to ZERO. " +
					"Will not find any compatible species.")
			}

			found := false
			var best_compatible *Species // the best compatible species
			best_compat_value := math.MaxFloat64
			for _, _specie := range pop.Species {
				// point _species
				if len(_specie.Organisms) > 0 {
					// point to first organism of this _specie
					compare_org := _specie.Organisms[0]
					// compare baby organism with first organism in current specie
					curr_compat := baby.GNome.compatibility(compare_org.GNome, context)

					if curr_compat < context.CompatThreshold && curr_compat < best_compat_value {
						best_compatible = _specie
						best_compat_value = curr_compat
						found = true
					}
				}
			}

			if found {
				neat.DebugLog(fmt.Sprintf("SPECIES: Compatible species [%d] found for baby organism [%d]",
					best_compatible.Id, baby.GNome.Id))
				// Found compatible species, so add this baby to it
				best_compatible.addOrganism(baby);
				// update in baby pointer to its species
				baby.SpeciesOf = best_compatible
			}

			// If match was not found, create a new species
			if !found {
				createFirstSpecies(pop, baby)
			}
		}

	} // end for count := 0
	return true, nil
}

func createFirstSpecies(pop *Population, baby *Organism) {
	neat.DebugLog(fmt.Sprintf("SPECIES: Create first species for baby organism [%d]", baby.GNome.Id))

	pop.LastSpecies++
	new_species := NewSpeciesNovel(pop.LastSpecies, true)
	pop.Species = append(pop.Species, new_species)
	new_species.addOrganism(baby) // Add the baby
	baby.SpeciesOf = new_species // Point baby to its species

	neat.DebugLog(fmt.Sprintf("SPECIES: # of species in population: %d, new species id: %d",
		len(pop.Species), new_species.Id))
}

func (s *Species) String() string {
	str := fmt.Sprintf("Species #%d, age=%d, avg_fitness=%.3f, max_fitness=%.3f, max_fitness_ever=%.3f, expected_offspring=%d, age_of_last_improvement=%d\n",
		s.Id, s.Age, s.AvgFitness, s.MaxFitness, s.MaxFitnessEver, s.ExpectedOffspring, s.AgeOfLastImprovement)
	str += fmt.Sprintf("Has %d Organisms:\n", len(s.Organisms))
	for _, o := range s.Organisms {
		str += fmt.Sprintf("\t%s\n", o)
	}
	return str
}

// This is used for list sorting of Species by fitness of best organism highest fitness first
// It implements sort.Interface for []Species based on the OriginalFitness of first Organism field in descending order,
// i.e. the max fitness goes first
type ByOrganismOrigFitness []*Species

func (f ByOrganismOrigFitness) Len() int {
	return len(f)
}
func (f ByOrganismOrigFitness) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}
func (f ByOrganismOrigFitness) Less(i, j int) bool {
	return f[i].Organisms[0].OriginalFitness > f[j].Organisms[0].OriginalFitness
}