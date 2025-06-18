package tts

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	"cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// Global TTS Client for reusability (Long Audio Synthesis).
var client *texttospeech.TextToSpeechLongAudioSynthesizeClient

func init() {
	var err error
	client, err = texttospeech.NewTextToSpeechLongAudioSynthesizeClient(context.Background())
	if err != nil {
		log.Fatalf("Failed to create Text-to-Speech Long Audio Synthesis client in internal/tts: %v", err)
	}
}

// SynthesizeLongAudio performs text-to-speech synthesis for long texts
// and outputs the audio directly to a GCS URI. It polls the operation until completion.
func SynthesizeLongAudio(ctx context.Context, text, projectNumber, location, outputGCSURI, voiceName string) error {
	req := texttospeechpb.SynthesizeLongAudioRequest{
		Input: &texttospeechpb.SynthesisInput{
			InputSource: &texttospeechpb.SynthesisInput_Text{Text: text},
		},
		AudioConfig: &texttospeechpb.AudioConfig{
			AudioEncoding:   texttospeechpb.AudioEncoding_LINEAR16, // Changed from MP3 to LINEAR16
			SampleRateHertz: 16000,                                 // LINEAR16 often requires a sample rate. 16kHz is common.
		},
		Voice: &texttospeechpb.VoiceSelectionParams{
			LanguageCode: "en-US",
			SsmlGender:   texttospeechpb.SsmlVoiceGender_NEUTRAL,
			Name:         voiceName,
		},
		OutputGcsUri: outputGCSURI,
		Parent:       fmt.Sprintf("projects/%s/locations/%s", projectNumber, location),
	}

	log.Println("Initiating Long Audio Synthesis...")
	op, err := client.SynthesizeLongAudio(ctx, &req)
	if err != nil {
		return fmt.Errorf("failed to initiate long audio synthesis: %w", err)
	}

	log.Printf("Long Audio Synthesis operation started: %s. Waiting for completion...", op.Name())

	for {
		latestOp, err := client.GetOperation(ctx, &longrunningpb.GetOperationRequest{Name: op.Name()})
		if err != nil {
			return fmt.Errorf("failed to get operation status for %s: %w", op.Name(), err)
		}

		if latestOp.Done {
			if latestOp.GetError() != nil {
				return fmt.Errorf("long audio synthesis operation failed for %s: %v", op.Name(), latestOp.GetError().Message)
			}
			var metadata texttospeechpb.SynthesizeLongAudioMetadata
			if latestOp.GetMetadata() != nil {
				if err := anypb.UnmarshalTo(latestOp.GetMetadata(), &metadata, proto.UnmarshalOptions{}); err != nil {
					log.Printf("Warning: Could not unmarshal operation metadata for %s: %v", op.Name(), err)
				} else {
					log.Printf("Long Audio Synthesis complete. Metadata: %s", &metadata)
				}
			}
			log.Printf("Long Audio Synthesis operation %s completed successfully.", op.Name())
			break
		}

		log.Printf("Operation %s not yet complete. Retrying in 10 seconds...", op.Name())
		time.Sleep(10 * time.Second)
	}

	return nil
}
