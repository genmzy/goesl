package ev_header

// --------------------- helper to event header -------------------------- //

// generic event headers
const (
	// Core uuid
	Core_Uuid = "Core-UUID"
	// event innate attributions
	Event_Name           = "Event-Name"
	Event_Sequence       = "Event-Sequence"
	FreeSWITCH_IPv4      = "FreeSWITCH-IPv4"
	Event_Date_Timestamp = "Event-Date-Timestamp"
	// background job related
	BgJob_Uuid        = "Job-Uuid"
	BgJob_Command     = "Job-Command"
	BgJob_Command_Arg = "Job-Command-Arg"
	// leg uuid
	Unique_ID           = "Unique-ID"
	Channel_State       = "Channel-State"
	Call_Direction      = "Call-Direction"
	Other_Leg_Unique_ID = "Other-Leg-Unique-ID"
	// caller
	Caller_ID_Number          = "Caller-Caller-ID-Number"
	Callee_Destination_Number = "Caller-Destination-Number"
	Caller_Network_Addr       = "Caller-Network-Addr"
	// digits
	DTMF_Digit  = "DTMF-Digit"
	DTMF_Source = "DTMF-Source"
	// hangup cause
	Hangup_Cause = "Hangup-Cause"
	// bridge a leg
	Bridge_A_Unique_ID = "Bridge-A-Unique-ID"
	// bridge b leg
	Bridge_B_Unique_ID = "Bridge-B-Unique-ID"
	// current application
	CurrentApp     = "variable_current_application"
	CurrentAppData = "variable_current_application_data"
	// application channel execute
	Application      = "Application"
	Application_Data = "Application-Data"
	// API
	API_Command          = "API-Command"
	API_Command_Argument = "API-Command-Argument"
	// profile
	Caller_Profile_Index = "Caller-Profile-Index"
	// record start/stop event
	Record_File_Path = "Record-File-Path"
	// media bug
	Media_Bug_Function = "Media-Bug-Function"
	Media_Bug_Target   = "Media-Bug-Target"
	// sip variables
	Sip_From_Host      = "variable_sip_from_host"
	Sip_To_Host        = "variable_sip_to_host"
	Sip_Call_ID        = "variable_sip_call_id"
	Sip_Profile_Name   = "variable_sip_profile_name"
	Sofia_Profile_Name = "variable_soofia_profile_name"
	Sip_Profile_Url    = "variable_sofia_profile_url"
	Sip_Req_Uri        = "variable_sip_req_uri"
	Sip_Network_Ip     = "variable_sip_network_ip"
	Sip_Network_Port   = "variable_sip_network_port"
	// general variables
	Local_Media_Ip        = "variable_local_media_ip"
	Local_Media_Port      = "variable_local_media_port"
	Rtp_Use_Codec_String  = "variable_rtp_use_codec_string"
	Rtp_Use_Codec_Name    = "variable_rtp_use_codec_name"
	Is_Outbound           = "vairable_is_outbound"
	Originate_Early_Media = "variable_originate_early_media"
	Remote_Media_Ip       = "variable_remote_media_ip"
	Remote_Media_Port     = "variable_remote_media_port"
	Dtmf_Type             = "variable_dtmf_type"
	Switch_R_Sdp          = "variable_switch_r_sdp"
	// leg summary vairables
	Start_UEpoch          = "variable_start_uepoch"
	Progress_UEpoch       = "variable_progress_uepoch"
	Progress_Media_UEpoch = "variable_progress_media_uepoch"
	Answer_UEpoch         = "variable_answer_uepoch"
	Bridge_UEpoch         = "variable_bridge_uepoch"
	End_UEpoch            = "variable_end_uepoch"

	Rtp_Audio_In_Packet_Count        = "variable_rtp_audio_in_packet_count"
	Rtp_Audio_In_Jitter_Packet_Count = "variable_rtp_audio_in_jitter_packet_count"
	Rtp_Audio_In_Jitter_Loss_Rate    = "variable_rtp_audio_in_jitter_loss_rate"
	Rtp_Audio_In_Quality_Percentage  = "variable_rtp_audio_in_quality_percentage"
	Rtp_Audio_Out_Packet_Count       = "variable_rtp_audio_out_packet_count"
)
