package normalizer

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const staticPayloadCanonicalRDataPatches = `
0:80b481765047817660498176d04581761039817610c6817650a8817630458176d03c817626a9b3
28:1fbcb3
2C:12aab3
30:20101f7519e6b3
38:10c91d75a0171f7560171f7550fa1e7520ca1d7580cc1d7530121f75104f1d75e0d21d7580ea1d7530eb1d7585d5b3
68:b04c1b77202a1b7780321a7769dbb3
78:00421a77b0d31d7520d41d7520da1d7550918f7340ea1d75e0db1d7580cd1d75101e1f7540341e7500e81d751bc6b3
A8:60e91d7510e91d7560bf1d7599b7b3
B8:c0e71d75f0411d75502b1f75e0432275f0651d75a8c4b3
D0:a9c0b3
D4:e6c2b3
D8:10501d7510d21d7590151f75c0b1b3
E8:8cc6b3
EC:c0ca1d75f0d61e7575b4b3
F8:92b2b3
FC:c0b21d7580d11d75200b1f75b02e1f752ea0b3
110:f01f1b77d0571b77e0fb1d75f5b6b3
120:60341e75004f1d7550c91d7500421d75a0f81877c0cc1d75b0411d7558b6b3
140:07e6b3
144:13e6b3
148:90908f7310ca1d7500d91d7580eb1d75001c1f75a0ca1d7530ee1d7526a9b3
168:20100010801100108ea9b3
174:f056b875b0a6b075905caf75603c207730afb0755085b075c0adb075f09fb075904ab8755054b87520e7b37510b2b075b0a9b075e2dbb3
1AC:30aeb07540b1b0751081b075a0e7b3752067b775a04bb8755052b8751056b075d0e3b775b068b775e083b075908fb07580e6b375d074b0751059b075306faf75d084b075c058b775e05cb775d039af7550b1b075706baf75607daf75d057b075909eb07510a4b075504bb0755091b675d0e4b775dba9b3
224:20392d7560fc2c75c0592c75909c2d7530272f7500022d7530782d75f0342d75a0762d75f06d2d7540712d75f0ae2d758ea9b3
258:3f1b7103d1d970036bfd7003cbf47003ff1971031b00710372fe700323f770037ff5700332fc7003d8b8700354bb7003bdc2700393c37003ecc1700330c970039ec8700372c2700356c470039bc4700377b670036fcf700366d570039bf47003f4a8b3
`

const staticPayloadCanonicalDataPatches = `
12A0:1b000000
12A8:17aa
12AC:c1a6
12B0:27a7
12B4:67a6
12B8:0fa7
12BC:17aa
1410:ffffffff
1430:ffffffff
1450:ffffffff
1460:1f70a8036901
1468:886fa80309
1470:03
1479:10
1490:04
1499:10
3820:3860a803
3860:e85fa803
387C:1017a803
3895:61a80310322105
400C:3e
4020:01
4028:ffffffffffffffff
403C:d007
403F:0204
4044:060805
404C:682f2105ac9ded04
4088:787a49
408C:1862a80313
40A8:8701
40E0:0f
40E2:09
40E4:a8fd2e
40F8:1e
40FA:1e
40FC:08
40FE:08
4104:ba3c12e608424376d2d769ec
4128:08
412A:200108
412E:21
4134:0f
4136:09
4138:a8fd2f
4158:08
415A:200108
415E:21
4160:0f
4162:09
4164:a8fd2f
416C:0a
416E:08
4170:05
4172:04
4180:6cd5a8031660
4188:80
4194:01
41B0:285a49
41B4:07
41B8:70c5a803
41C0:01
41C2:07
41C4:01
41C8:07
41D0:285a49
41D4:05
41D8:40c5a803e4c9f90401
41E2:05
41E4:01
41E8:05
41F0:285a49
41F4:0a
41F8:f0c0a803d44c08050a
4202:01
4208:0a
4210:285a49
422C:285a49
4248:285a49
4264:285a49
4280:285a49
42A0:285a49
42A4:0e
42A8:10b9a8030c13070501
42B2:0e
42B4:01
42B8:0e
42BC:285a49
42C0:0c
42C4:70b9a803
42CC:01
42CE:0c
42D0:01
42D4:0c
42D8:285a49
42DC:0c
42E0:b0b8a803
42E8:01
42EA:0c
42EC:01
42F0:0c
42F4:285a49
42F8:0c
42FC:40b9a803
4304:01
4306:0c
4308:01
430C:0c
4310:285a49
4314:0c
4318:80b8a803
4320:01
4322:0c
4324:01
4328:0c
432C:285a49
4330:0a
4334:e0b8a803
433C:01
433E:0a
4340:01
4344:0a
4348:285a49
434C:08
4350:f0b7a803
4358:01
435A:08
435C:01
4360:08
4364:285a49
4368:0c
436C:50b8a803
4374:01
4376:0c
4378:01
437C:0c
4380:285a49
4384:0e
4388:a0b9a803
4390:01
4392:0e
4394:01
4398:0e
439C:285a49
43A0:0b
43A4:20b8a803
43AC:01
43AE:0b
43B0:01
43B4:0b
43B8:285a49
43BC:06
43C0:b0caa803
43C8:01
43CA:06
43CC:01
43D0:06
43D8:285a49
43DC:06
43E0:80caa803eccaf90401
43EA:06
43EC:01
43F0:06
43F8:607c49
43FC:07
4400:80bba8038c71280507
440A:01
4410:607c49
4414:04
4418:c0c0a803c467e60404
4422:01
4428:607c49
442C:0a
4430:d0b9a803d42419050a
443A:01
4440:607c49
4454:607c49
4468:607c49
447C:607c49
4490:607c49
44A8:607c49
44AC:04
44B0:90cca803
44B8:01
44BA:04
44BC:607c49
44C0:04
44C4:60cca803
44CC:01
44CE:04
44D0:607c49
44D4:04
44D8:c0cca803
44E0:01
44E2:04
44E4:607c49
44E8:04
44EC:50cda803
44F4:01
44F6:04
44F8:607c49
44FC:04
4500:f0cca803
4508:01
450A:04
450C:607c49
4510:04
4514:20cda803
451C:01
451E:04
4520:607c49
4524:08
4528:70cba803
4530:01
4532:08
4534:607c49
4538:08
453C:40cba803
4544:01
4546:08
4548:607c49
454C:08
4550:a0cba803
4558:01
455A:08
455C:607c49
4560:08
4564:30cca803
456C:01
456E:08
4570:607c49
4574:08
4578:d0cba803
4580:01
4582:08
4584:607c49
4588:08
458D:cca803
4594:01
4596:08
4598:607c49
459C:07
45A0:10c5a80304f6840301
45AA:07
45B0:607c49
45B4:06
45B8:20c4a80334e5f40406
45C2:01
45C8:607c49
45CC:04
45D0:c0cfa803a478e60404
45DA:01
45E0:607c49
45E4:09
45E8:a0d4a80314c5850301
45F2:09
45F8:607c49
45FC:13
4600:90b7a803ac3d190513
460A:01
4610:607c49
4614:0c
4618:10cba8033cfa04050c
4622:01
4628:607c49
4640:607c49
4644:18
4648:e0d3a803
4650:18
4652:01
4658:607c49
4670:607c49
4674:04
4678:f0c3a803
4680:01
4682:04
4688:607c49
468C:02
4690:c0c3a80324091d0501
469A:02
46A0:607c49
46A4:06
46A8:a0c2a8037c85800301
46B2:06
46B8:607c49
46BC:05
46C0:60aba803acbd820305
46CA:01
46D0:607c49
46D4:08
46D8:d0aaa80314bc820301
46E2:08
46E8:607c49
46EC:1e
46F0:b0c7a80334fd7e031e
46FA:01
4700:607c49
4704:07
4708:c0c9a8030cade40407
4712:01
4718:01
4724:2861a803
472C:0801
4734:01
4738:e05da803
4740:8c3a7803
4760:2c
4768:01
476C:ffffffffffffffffffffffffffffffff2d02
4780:d6
4784:5305
4788:4b03
4794:0404
4798:4cc2
47A0:01
47A2:2003580210
47A8:c3dd8e84
47B0:4c71ed04583a7803583a78032066a80378ff29052a02
47C8:4002
47CC:20
47D4:e8797703e8cb7803907fa803
47E4:f7ad0401c887a803
47FE:ca82a0b4ad
4804:583a78032066a803e8797703907fa803885ba803080705
489C:6c16f304
4C20:b824
4C24:6023
4C28:2404
4C2C:0c04
4C36:ca96
4E48:ffffffffffffffff
4E5C:d007
4E5F:0202
4E7C:f023
4E80:0206
4E84:06
4E88:02
4E8C:01
4E90:3817a803
4E98:9017a803
4EA8:bc0e4a
4EBC:433a5c50726f6772616d2046696c65732028783836295c4c656d6f6e616465205479636f6f6e2032202d204e657720596f726b20436974795c4c656d6f6e616465322e657865
4FC0:01
4FC8:0d
4FD0:ffffffffffffffff
4FE4:d007
4FE7:02ffffffffffffffff
4FFC:d007
4FFF:02ffffffffffffffff
5014:d007
5017:02ffffffffffffffff
502C:d007
502F:02244a0110
503C:01
5040:01
5094:01
51D8:0101
51E4:e404
5241:6162636465666768696a6b6c6d6e6f707172737475767778797a
5261:4142434445464748494a4b4c4d4e4f505152535455565758595a
5283:83
528A:9a
528C:9c
528E:9e
529A:8a
529C:8c
529E:8eff
52AA:aa
52B5:b5
52BA:ba
52C0:e0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6
52D8:f8f9fafbfcfdfedfc0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6
52F8:d8d9dadbdcddde9f
5342:1010101010101010101010101010101010101010101010101010
5362:2020202020202020202020202020202020202020202020202020
5384:20
538B:10
538D:10
538F:10
539B:20
539D:20
539F:2010
53AB:20
53B6:20
53BB:20
53C1:1010101010101010101010101010101010101010101010
53D9:10101010101010202020202020202020202020202020202020202020202020
53F9:2020202020202020
5424:a866a803
6441:02
6446:a80301
6460:4006a803
6560:20
6564:01
6568:01
656C:746fa803b06ea803385494
`

func applyStaticPayloadPatches(section []byte, encoded string) error {
	patches, err := parseStaticPayloadPatches(encoded)
	if err != nil {
		return err
	}
	for _, patch := range patches {
		start := patch.Offset
		data := patch.Data
		if start+len(data) > len(section) {
			return fmt.Errorf("patch 0x%X..0x%X exceeds section size 0x%X", start, start+len(data), len(section))
		}
		copy(section[start:start+len(data)], data)
	}
	return nil
}

type staticPayloadPatch struct {
	Offset int
	Data   []byte
}

func parseStaticPayloadPatches(encoded string) ([]staticPayloadPatch, error) {
	var patches []staticPayloadPatch
	for line := range strings.FieldsSeq(encoded) {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad patch line %q", line)
		}
		offset, err := strconv.ParseUint(parts[0], 16, 32)
		if err != nil {
			return nil, err
		}
		data, err := hex.DecodeString(parts[1])
		if err != nil {
			return nil, err
		}
		patches = append(patches, staticPayloadPatch{Offset: int(offset), Data: data})
	}
	return patches, nil
}

type portableDataPatchSummary struct {
	AppliedDwords         int
	AppliedBytes          int
	SkippedArtifactDwords int
	SkippedArtifactBytes  int
}

func summarizeSkippedRDataPatches() (portablePatchSummary, error) {
	patches, err := parseStaticPayloadPatches(staticPayloadCanonicalRDataPatches)
	if err != nil {
		return portablePatchSummary{}, err
	}
	summary := portablePatchSummary{SkippedRDataRanges: len(patches)}
	for _, patch := range patches {
		summary.SkippedRDataBytes += len(patch.Data)
	}
	return summary, nil
}

func applyPortableDataPatches(section []byte) (portableDataPatchSummary, error) {
	patches, err := parseStaticPayloadPatches(staticPayloadCanonicalDataPatches)
	if err != nil {
		return portableDataPatchSummary{}, err
	}
	patchedBytes := make(map[int]byte)
	for _, patch := range patches {
		if patch.Offset+len(patch.Data) > len(section) {
			return portableDataPatchSummary{}, fmt.Errorf("patch 0x%X..0x%X exceeds section size 0x%X", patch.Offset, patch.Offset+len(patch.Data), len(section))
		}
		for index, value := range patch.Data {
			patchedBytes[patch.Offset+index] = value
		}
	}

	summary := portableDataPatchSummary{}
	for dwordOffset := range touchedDwords(patchedBytes) {
		if dwordOffset+4 > len(section) {
			continue
		}
		after := append([]byte(nil), section[dwordOffset:dwordOffset+4]...)
		for index := range after {
			if value, ok := patchedBytes[dwordOffset+index]; ok {
				after[index] = value
			}
		}
		patchBytes := patchedDwordByteCount(patchedBytes, dwordOffset)
		if isCanonicalDumpArtifactDataDword(dwordOffset, binary.LittleEndian.Uint32(after)) {
			summary.SkippedArtifactDwords++
			summary.SkippedArtifactBytes += patchBytes
			continue
		}
		summary.AppliedDwords++
		summary.AppliedBytes += patchBytes
		for index, value := range after {
			if _, ok := patchedBytes[dwordOffset+index]; ok {
				section[dwordOffset+index] = value
			}
		}
	}
	return summary, nil
}

func touchedDwords(patchedBytes map[int]byte) map[int]struct{} {
	touched := make(map[int]struct{})
	for offset := range patchedBytes {
		touched[offset&^3] = struct{}{}
	}
	return touched
}

func patchedDwordByteCount(patchedBytes map[int]byte, dwordOffset int) int {
	count := 0
	for offset := dwordOffset; offset < dwordOffset+4; offset++ {
		if _, ok := patchedBytes[offset]; ok {
			count++
		}
	}
	return count
}

func isCanonicalDumpArtifactDataDword(offset int, value uint32) bool {
	if offset == 0x4C34 || offset == 0x4FC8 {
		return true
	}
	return value >= 0x01000000 && value < 0x10000000
}
