package cloud_test

import (
	"errors"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	biproperty "github.com/cloudfoundry/bosh-utils/property"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakebicloud "github.com/cloudfoundry/bosh-cli/cloud/fakes"

	. "github.com/cloudfoundry/bosh-cli/cloud"
)

var _ = Describe("Cloud", func() {
	var (
		cloud               Cloud
		expectedContext     CmdContext
		fakeCPICmdRunner    *fakebicloud.FakeCPICmdRunner
		logger              boshlog.Logger
		stemcellApiVersion  interface{} = 1
		infoResult          map[string]interface{}
		infoResultWithApiV2 map[string]interface{}
	)

	BeforeEach(func() {
		fakeCPICmdRunner = fakebicloud.NewFakeCPICmdRunner()
		logger = boshlog.NewLogger(boshlog.LevelNone)
		infoResult = map[string]interface{}{
			"stemcell_formats": []interface{}{"aws-raw", "aws-light"},
		}
		infoResultWithApiV2 = map[string]interface{}{
			"stemcell_formats": []interface{}{"aws-raw", "aws-light"},
			"api_version":      float64(2),
		}
	})

	JustBeforeEach(func() {
		expectedContext = CmdContext{DirectorID: "fake-director-id", StemcellApiVersion: stemcellApiVersion.(int)}
		cloud = NewCloud(fakeCPICmdRunner, "fake-director-id", stemcellApiVersion.(int), logger)
	})

	var itHandlesCPIErrors = func(method string, callsInfo bool, exec func() error) {
		It("returns a cloud.Error when the CPI command returns an error", func() {

			fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{}

			fakeCPICmdRunner.RunCmdOutputs = append(
				fakeCPICmdRunner.RunCmdOutputs,
				CmdOutput{
					Error: &CmdError{
						Type:    "Bosh::Cloud::CloudError",
						Message: "fake-cpi-error-msg",
					},
				},
			)

			if callsInfo {
				fakeCPICmdRunner.RunCmdOutputs = append(
					fakeCPICmdRunner.RunCmdOutputs,
					CmdOutput{Result: infoResult},
				)
			}

			err := exec()
			Expect(err).To(HaveOccurred())

			cpiError, ok := err.(Error)
			Expect(ok).To(BeTrue(), "Expected %s to implement the Error interface", cpiError)
			Expect(cpiError.Method()).To(Equal(method))
			Expect(cpiError.Type()).To(Equal("Bosh::Cloud::CloudError"))
			Expect(cpiError.Message()).To(Equal("fake-cpi-error-msg"))
			Expect(err.Error()).To(ContainSubstring("Bosh::Cloud::CloudError"))
			Expect(err.Error()).To(ContainSubstring("fake-cpi-error-msg"))
		})
	}

	Describe("Info", func() {

		var itReturnsAValidDefaultCpiInfo = func() {
			It("returns a valid cpi info", func() {
				cpiInfo := cloud.Info()
				Expect(cpiInfo).To(Equal(CpiInfo{
					StemcellFormats: []string{},
					ApiVersion:      1,
				}))
			})
		}

		Context("when the stemcell version is 2", func() {
			BeforeEach(func() {
				stemcellApiVersion = 2
			})

			It("return info based on cpi", func() {
				infoParsed := CpiInfo{
					StemcellFormats: []string{"aws-raw", "aws-light"},
					ApiVersion:      2,
				}
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
					Result: infoResultWithApiV2,
				}}
				cpiInfo := cloud.Info()
				Expect(cpiInfo).To(Equal(infoParsed))

				Expect(fakeCPICmdRunner.CurrentRunInput).To(Equal([]fakebicloud.RunInput{
					{
						Context:   expectedContext,
						Method:    "info",
						Arguments: []interface{}{" "},
					},
				}))
			})

			It("uses a default cpi api version if an old cpi does not have api version", func() {
				infoParsed := CpiInfo{
					StemcellFormats: []string{"aws-raw", "aws-light"},
					ApiVersion:      1,
				}
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
					Result: infoResult,
				}}
				cpiInfo := cloud.Info()
				Expect(cpiInfo).To(Equal(infoParsed))
			})

			Context("when the cpi command execution fails", func() {
				BeforeEach(func() {
					fakeCPICmdRunner.RunErrs = []error{errors.New("info")}
				})

				itReturnsAValidDefaultCpiInfo()
			})

			Context("when the cpi version is > 2", func() {
				It("should return MAX supported version by CLI", func() {
					infoResult = map[string]interface{}{
						"stemcell_formats": []interface{}{"aws-raw", "aws-light"},
						"api_version":      float64(42),
					}
					infoParsed := CpiInfo{
						StemcellFormats: []string{"aws-raw", "aws-light"},
						ApiVersion:      2,
					}
					fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
						Result: infoResult,
					}}
					cpiInfo := cloud.Info()
					Expect(cpiInfo).To(Equal(infoParsed))
				})
			})

			Context("when info return unexpected format result", func() {
				Context("when api_version is not a number format", func() {
					BeforeEach(func() {
						infoResultWithApiV2 = map[string]interface{}{
							"stemcell_formats": []interface{}{"aws-raw", "aws-light"},
							"api_version":      "57",
						}

						fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
							Result: infoResultWithApiV2,
						}}
					})

					itReturnsAValidDefaultCpiInfo()
				})
				Context("when stemcell formats is not a []string", func() {
					BeforeEach(func() {
						infoResultWithApiV2 = map[string]interface{}{
							"stemcell_formats": "aws-raw",
							"api_version":      stemcellApiVersion,
						}
						fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
							Result: infoResultWithApiV2,
						}}
					})

					itReturnsAValidDefaultCpiInfo()
				})
			})
		})
	})

	Describe("CreateStemcell", func() {
		var (
			stemcellImagePath string
			cloudProperties   biproperty.Map
		)

		BeforeEach(func() {
			stemcellImagePath = "/stemcell/path"
			cloudProperties = biproperty.Map{
				"fake-key": "fake-value",
			}
		})

		Context("when the cpi successfully creates the stemcell", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
					Result: "fake-cid",
				}}
			})

			It("executes the cpi job script with stemcell image path & cloud_properties", func() {
				_, err := cloud.CreateStemcell(stemcellImagePath, cloudProperties)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "create_stemcell",
					Arguments: []interface{}{
						stemcellImagePath,
						cloudProperties,
					},
				}))
			})

			It("returns the cid returned from executing the cpi script", func() {
				cid, err := cloud.CreateStemcell(stemcellImagePath, cloudProperties)
				Expect(err).NotTo(HaveOccurred())
				Expect(cid).To(Equal("fake-cid"))
			})
		})

		Context("when the result is of an unexpected type", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
					Result: 1,
				}}
			})

			It("returns an error", func() {
				_, err := cloud.CreateStemcell(stemcellImagePath, cloudProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected external CPI command result: '1'"))
			})
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				_, err := cloud.CreateStemcell(stemcellImagePath, cloudProperties)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("create_stemcell", false, func() error {
			_, err := cloud.CreateStemcell(stemcellImagePath, cloudProperties)
			return err
		})
	})

	Describe("DeleteStemcell", func() {
		It("executes the delete_stemcell method on the CPI with stemcell cid", func() {
			err := cloud.DeleteStemcell("fake-stemcell-cid")
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
			Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
				Context: expectedContext,
				Method:  "delete_stemcell",
				Arguments: []interface{}{
					"fake-stemcell-cid",
				},
			}))
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				err := cloud.DeleteStemcell("fake-stemcell-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("delete_stemcell", false, func() error {
			return cloud.DeleteStemcell("fake-stemcell-cid")
		})
	})

	Describe("HasVM", func() {
		It("return true when VM exists", func() {
			fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
				Result: true,
			}}

			found, err := cloud.HasVM("fake-vm-cid")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
				Context:   expectedContext,
				Method:    "has_vm",
				Arguments: []interface{}{"fake-vm-cid"},
			}))
		})

		It("return false when VM does not exist", func() {
			fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
				Result: false,
			}}

			found, err := cloud.HasVM("fake-vm-cid")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error when executing the CPI command fails", func() {
				_, err := cloud.HasVM("fake-vm-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("has_vm", false, func() error {
			_, err := cloud.HasVM("fake-vm-cid")
			return err
		})
	})

	Describe("CreateVM", func() {
		var (
			agentID           string
			stemcellCID       string
			cloudProperties   biproperty.Map
			networkInterfaces map[string]biproperty.Map
			env               biproperty.Map
		)

		BeforeEach(func() {
			agentID = "fake-agent-id"
			stemcellCID = "fake-stemcell-cid"
			networkInterfaces = map[string]biproperty.Map{
				"bosh": {
					"type": "dynamic",
					"cloud_properties": biproperty.Map{
						"a": "b",
					},
				},
			}
			cloudProperties = biproperty.Map{
				"fake-cloud-property-key": "fake-cloud-property-value",
			}
			env = biproperty.Map{
				"fake-env-key": "fake-env-value",
			}
		})

		Context("when the cpi successfully creates the vm", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{
					{
						Result: "fake-vm-cid",
					},
					{
						Result: infoResult,
					},
				}
			})

			It("executes the cpi job script with the director UUID and stemcell CID", func() {
				_, err := cloud.CreateVM(agentID, stemcellCID, cloudProperties, networkInterfaces, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(2))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "create_vm",
					Arguments: []interface{}{
						agentID,
						stemcellCID,
						cloudProperties,
						networkInterfaces,
						[]interface{}{},
						env,
					},
				}))
			})

			It("returns the cid returned from executing the cpi script", func() {
				cid, err := cloud.CreateVM(agentID, stemcellCID, cloudProperties, networkInterfaces, env)
				Expect(err).NotTo(HaveOccurred())
				Expect(cid).To(Equal("fake-vm-cid"))
			})

			Context("when stemcell api_version is 2", func() {
				BeforeEach(func() {
					stemcellApiVersion = 2
				})

				Context("when cpi api_version is 2", func() {
					BeforeEach(func() {
						fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{
							{
								Result: []string{"fake-vm-cid", "network-hash"},
							},
							{
								Result: infoResultWithApiV2,
							},
						}
					})

					It("returns the vm cid", func() {
						cid, err := cloud.CreateVM(agentID, stemcellCID, cloudProperties, networkInterfaces, env)
						Expect(err).NotTo(HaveOccurred())
						Expect(cid).To(Equal("fake-vm-cid"))
					})
				})
			})
		})

		Context("when the result is of an unexpected type", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{
					{
						Result: 1,
					},
				}
			})

			It("returns an error", func() {
				_, err := cloud.CreateVM(agentID, stemcellCID, cloudProperties, networkInterfaces, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected external CPI command result: '1'"))
			})
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				_, err := cloud.CreateVM(agentID, stemcellCID, cloudProperties, networkInterfaces, env)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("create_vm", true, func() error {
			_, err := cloud.CreateVM(agentID, stemcellCID, cloudProperties, networkInterfaces, env)
			return err
		})

	})

	Describe("SetDiskMetadata", func() {
		metadata := DiskMetadata{
			"director":       "bosh-init",
			"deployment":     "some-deployment",
			"instance_group": "some-instance_group",
			"instance_index": "0",
			"attached_at":    "2017-03-22T10:17:04Z",
		}
		It("calls the set_disk_metadata CPI method", func() {
			diskCID := "fake-disk-cid"
			err := cloud.SetDiskMetadata(diskCID, metadata)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
			Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
				Context: expectedContext,
				Method:  "set_disk_metadata",
				Arguments: []interface{}{
					diskCID,
					metadata,
				},
			}))
		})

		It("returns the error if running fails", func() {
			fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			diskCID := "fake-disk-cid"
			err := cloud.SetDiskMetadata(diskCID, metadata)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake-run-error"))
		})

		itHandlesCPIErrors("set_disk_metadata", false, func() error {
			diskCID := "fake-disk-cid"
			return cloud.SetDiskMetadata(diskCID, metadata)
		})
	})

	Describe("SetVMMetadata", func() {
		It("calls the set_vm_metadata CPI method", func() {
			vmCID := "fake-vm-cid"
			metadata := VMMetadata{
				"director":   "bosh-init",
				"deployment": "some-deployment",
				"job":        "some-job",
				"index":      "0",
			}
			err := cloud.SetVMMetadata(vmCID, metadata)
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
			Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
				Context: expectedContext,
				Method:  "set_vm_metadata",
				Arguments: []interface{}{
					vmCID,
					metadata,
				},
			}))
		})

		It("returns the error if running fails", func() {
			fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			vmCID := "fake-vm-cid"
			metadata := VMMetadata{
				"director":   "bosh-init",
				"deployment": "some-deployment",
				"job":        "some-job",
				"index":      "0",
			}

			err := cloud.SetVMMetadata(vmCID, metadata)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("fake-run-error"))
		})

		itHandlesCPIErrors("set_vm_metadata", false, func() error {
			vmCID := "fake-vm-cid"
			metadata := VMMetadata{
				"director":   "bosh-init",
				"deployment": "some-deployment",
				"job":        "some-job",
				"index":      "0",
			}
			return cloud.SetVMMetadata(vmCID, metadata)
		})
	})

	Describe("CreateDisk", func() {
		var (
			size            int
			cloudProperties biproperty.Map
			instanceID      string
		)

		BeforeEach(func() {
			size = 1024
			cloudProperties = biproperty.Map{
				"fake-cloud-property-key": "fake-cloud-property-value",
			}
			instanceID = "fake-instance-id"
		})

		Context("when the cpi successfully creates the disk", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
					Result: "fake-disk-cid",
				}}
			})

			It("executes the cpi job script with the correct arguments", func() {
				_, err := cloud.CreateDisk(size, cloudProperties, instanceID)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "create_disk",
					Arguments: []interface{}{
						size,
						cloudProperties,
						instanceID,
					},
				}))
			})

			It("returns the cid returned from executing the cpi script", func() {
				cid, err := cloud.CreateDisk(size, cloudProperties, instanceID)
				Expect(err).NotTo(HaveOccurred())
				Expect(cid).To(Equal("fake-disk-cid"))
			})
		})

		Context("when the result is of an unexpected type", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{{
					Result: 1,
				}}
			})

			It("returns an error", func() {
				_, err := cloud.CreateDisk(size, cloudProperties, instanceID)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unexpected external CPI command result: '1'"))
			})
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				_, err := cloud.CreateDisk(size, cloudProperties, instanceID)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("create_disk", false, func() error {
			_, err := cloud.CreateDisk(size, cloudProperties, instanceID)
			return err
		})
	})

	Describe("AttachDisk", func() {
		Context("when stemcell api version and cpi api version are 2", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{
					{Result: "/dev/sdf"},
					{Result: infoResultWithApiV2},
				}
				stemcellApiVersion = 2
			})

			It("executes the cpi job script with the correct arguments", func() {
				diskHint, err := cloud.AttachDisk("fake-vm-cid", "fake-disk-cid")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(2))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "attach_disk",
					Arguments: []interface{}{
						"fake-vm-cid",
						"fake-disk-cid",
					},
				}))
				Expect(diskHint).To(Equal("/dev/sdf"))
			})

		})

		Context("when the cpi successfully attaches the disk", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunCmdOutputs = []CmdOutput{
					{Result: infoResult},
				}
			})

			It("executes the cpi job script with the correct arguments", func() {
				_, err := cloud.AttachDisk("fake-vm-cid", "fake-disk-cid")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(2))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "attach_disk",
					Arguments: []interface{}{
						"fake-vm-cid",
						"fake-disk-cid",
					},
				}))
			})
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				_, err := cloud.AttachDisk("fake-vm-cid", "fake-disk-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("attach_disk", true, func() error {
			_, err := cloud.AttachDisk("fake-vm-cid", "fake-disk-cid")
			return err
		})
	})

	Describe("DetachDisk", func() {
		Context("when the cpi successfully detaches the disk", func() {
			It("executes the cpi job script with the correct arguments", func() {
				err := cloud.DetachDisk("fake-vm-cid", "fake-disk-cid")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "detach_disk",
					Arguments: []interface{}{
						"fake-vm-cid",
						"fake-disk-cid",
					},
				}))
			})
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				err := cloud.DetachDisk("fake-vm-cid", "fake-disk-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("detach_disk", false, func() error {
			return cloud.DetachDisk("fake-vm-cid", "fake-disk-cid")
		})
	})

	Describe("DeleteVM", func() {
		Context("when the cpi successfully deletes vm", func() {
			It("executes the cpi job script with the correct arguments", func() {
				err := cloud.DeleteVM("fake-vm-cid")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "delete_vm",
					Arguments: []interface{}{
						"fake-vm-cid",
					},
				}))
			})
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				err := cloud.DeleteVM("fake-vm-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("delete_vm", false, func() error {
			return cloud.DeleteVM("fake-vm-cid")
		})
	})

	Describe("DeleteDisk", func() {
		Context("when the cpi successfully deletes disk", func() {
			It("executes the cpi job script with the correct arguments", func() {
				err := cloud.DeleteDisk("fake-disk-cid")
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCPICmdRunner.CurrentRunInput).To(HaveLen(1))
				Expect(fakeCPICmdRunner.CurrentRunInput[0]).To(Equal(fakebicloud.RunInput{
					Context: expectedContext,
					Method:  "delete_disk",
					Arguments: []interface{}{
						"fake-disk-cid",
					},
				}))
			})
		})

		Context("when the cpi command execution fails", func() {
			BeforeEach(func() {
				fakeCPICmdRunner.RunErrs = []error{errors.New("fake-run-error")}
			})

			It("returns an error", func() {
				err := cloud.DeleteDisk("fake-disk-cid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-run-error"))
			})
		})

		itHandlesCPIErrors("delete_disk", false, func() error {
			return cloud.DeleteDisk("fake-disk-cid")
		})
	})
})
